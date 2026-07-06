import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiClient } from '../api/client';

export default function Dashboard() {
    const [campaigns, setCampaigns] = useState([]);
    const [selectedCampaign, setSelectedCampaign] = useState(null);
    const [ads, setAds] = useState([]);
    const [stats, setStats] = useState({});
    const navigate = useNavigate();
    const wsRef = useRef(null);

    const [showCampaignForm, setShowCampaignForm] = useState(false);
    const [newCampaign, setNewCampaign] = useState({ name: '', start_date: '', end_date: '' });

    const [showAdForm, setShowAdForm] = useState(false);
    const [newAd, setNewAd] = useState({ context_features: '{"target_age": "18-25", "device": "desktop"}', image: null });
    const [editingCampaign, setEditingCampaign] = useState(null);


    const formatForInput = (isoString) => {
        if (!isoString) return '';
        const date = new Date(isoString);
        return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
    };

    const fetchCampaigns = () => {
        apiClient.get('/campaigns')
            .then(res => setCampaigns(res.data || []))
            .catch(err => {
                if (err.response?.status === 401) {
                    localStorage.removeItem('jwt_token');
                    navigate('/login');
                }
            });
    };

    useEffect(() => {
        fetchCampaigns();
    }, [navigate]);

    useEffect(() => {
        if (!selectedCampaign) return;

        const fetchDetails = async () => {
            try {
                const [adsRes, statsRes] = await Promise.all([
                    apiClient.get(`/campaigns/${selectedCampaign.id}/ads`),
                    apiClient.get(`/campaigns/${selectedCampaign.id}/stats`)
                ]);

                setAds(adsRes.data || []);
                const statsMap = {};
                (statsRes.data || []).forEach(s => { statsMap[s.ad_id] = s; });
                setStats(statsMap);
            } catch (err) {
                console.error("Błąd pobierania detali kampanii", err);
            }
        };

        fetchDetails();

        const ws = new WebSocket(`wss://${window.location.host}/api/v1/ws`);
        wsRef.current = ws;
        ws.onopen = () => console.log('Połączono ze strumieniem WebSocket');
        ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);

            // Backend wysyła teraz różne typy wiadomości (event, campaign_created, ad_created, ad_deleted).
            // Dashboard aktualizuje statystyki tylko dla wiadomości typu "event".
            if (msg.type !== 'event') return;

            const newEvent = msg.payload;
            setStats(prevStats => {
                const currentAdStats = prevStats[newEvent.ad_id] || { impressions: 0, clicks: 0, ctr: 0 };
                const newImpressions = currentAdStats.impressions + (newEvent.event_type === 'impression' ? 1 : 0);
                const newClicks = currentAdStats.clicks + (newEvent.event_type === 'click' ? 1 : 0);
                const newCtr = newImpressions > 0 ? (newClicks / newImpressions).toFixed(4) : 0;

                return {
                    ...prevStats,
                    [newEvent.ad_id]: { ...currentAdStats, impressions: newImpressions, clicks: newClicks, ctr: newCtr }
                };
            });
        };

        return () => { if (wsRef.current) wsRef.current.close(); };
    }, [selectedCampaign]);


    const handleCreateCampaign = async (e) => {
        e.preventDefault();
        try {
            const payload = {
                ...newCampaign,
                start_date: new Date(newCampaign.start_date).toISOString(),
                end_date: new Date(newCampaign.end_date).toISOString()
            };
            await apiClient.post('/campaigns', payload);
            fetchCampaigns(); // Odśwież listę
            setShowCampaignForm(false);
            setNewCampaign({ name: '', start_date: '', end_date: '' });
        } catch (err) {
            alert("Błąd podczas tworzenia kampanii: " + (err.response?.data || err.message));
        }
    };
    const handleUpdateCampaign = async (e) => {
        e.preventDefault();
        try {
            const payload = {
                name: editingCampaign.name,
                start_date: new Date(editingCampaign.start_date).toISOString(),
                end_date: new Date(editingCampaign.end_date).toISOString()
            };
            await apiClient.put(`/campaigns/${editingCampaign.id}`, payload);
            fetchCampaigns();
            setEditingCampaign(null);
            if (selectedCampaign && selectedCampaign.id === editingCampaign.id) {
                setSelectedCampaign({ ...selectedCampaign, name: editingCampaign.name });
            }
        } catch (err) {
            alert("Błąd podczas aktualizacji kampanii: " + (err.response?.data || err.message));
        }
    };
    const handleCreateAd = async (e) => {
        e.preventDefault();
        if (!selectedCampaign) return alert("Wybierz kampanię z listy!");
        if (!newAd.image) return alert("Wybierz plik graficzny!");

        try {
            JSON.parse(newAd.context_features);

            const formData = new FormData();
            formData.append('campaign_id', selectedCampaign.id);
            formData.append('context_features', newAd.context_features);
            formData.append('image', newAd.image);

            await apiClient.post('/ads', formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            });

            const adsRes = await apiClient.get(`/campaigns/${selectedCampaign.id}/ads`);
            setAds(adsRes.data || []);
            setShowAdForm(false);
            setNewAd({ ...newAd, image: null });
        } catch (err) {
            alert("Błąd: Upewnij się, że JSON jest poprawny. Szegóły: " + (err.response?.data || err.message));
        }
    };
    const handleDeleteAd = async (adId) => {
        if (!window.confirm("Czy na pewno chcesz usunąć ten baner? To usunie również jego statystyki.")) return;

        try {
            await apiClient.delete(`/ads/${adId}`);

            setAds(prevAds => prevAds.filter(ad => ad.id !== adId));
        } catch (err) {
            alert("Błąd podczas usuwania reklamy: " + (err.response?.data || err.message));
        }
    };

    return (
        <div style={{ maxWidth: '1200px', margin: '40px auto', fontFamily: 'sans-serif', display: 'flex', gap: '30px' }}>

            <div style={{ width: '300px', borderRight: '2px solid #eee', paddingRight: '20px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <h2>Kampanie</h2>
                    <button onClick={() => { localStorage.removeItem('jwt_token'); navigate('/login'); }} style={{ padding: '5px 10px', background: '#dc3545', color: 'white', border: 'none', cursor: 'pointer' }}>
                        Wyloguj
                    </button>
                </div>
                <div style={{ marginBottom: '20px', paddingBottom: '15px', borderBottom: '1px solid #ddd' }}>
                    <h3 style={{ marginTop: 0, fontSize: '1rem', color: '#666' }}>Nawigacja Systemowa</h3>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                        <button
                            onClick={() => navigate('/admin/stats')}
                            style={{ padding: '10px', background: '#3b82f6', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', textAlign: 'left', fontWeight: 'bold' }}>
                            📊 Obserwowalność API
                        </button>
                        <button
                            onClick={() => window.open('/store', '_blank')}
                            style={{ padding: '10px', background: '#6366f1', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer', textAlign: 'left', fontWeight: 'bold' }}>
                            🛒 Otwórz Sklep (Nowa karta)
                        </button>
                    </div>
                </div>
                <button onClick={() => setShowCampaignForm(!showCampaignForm)} style={{ width: '100%', padding: '10px', margin: '10px 0', background: '#28a745', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>
                    {showCampaignForm ? "Anuluj" : "+ Nowa Kampania"}
                </button>

                {showCampaignForm && (
                    <form onSubmit={handleCreateCampaign} style={{ display: 'flex', flexDirection: 'column', gap: '10px', padding: '15px', background: '#f8f9fa', borderRadius: '8px', marginBottom: '15px' }}>
                        <input type="text" placeholder="Nazwa kampanii" required value={newCampaign.name} onChange={e => setNewCampaign({...newCampaign, name: e.target.value})} style={{ padding: '8px' }}/>
                        <label style={{ fontSize: '0.8em' }}>Data startu:</label>
                        <input type="datetime-local" required value={newCampaign.start_date} onChange={e => setNewCampaign({...newCampaign, start_date: e.target.value})} style={{ padding: '8px' }}/>
                        <label style={{ fontSize: '0.8em' }}>Data końca:</label>
                        <input type="datetime-local" required value={newCampaign.end_date} onChange={e => setNewCampaign({...newCampaign, end_date: e.target.value})} style={{ padding: '8px' }}/>
                        <button type="submit" style={{ padding: '8px', background: '#007bff', color: 'white', border: 'none' }}>Utwórz</button>
                    </form>
                )}

                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                    {campaigns.map(camp => (
                        <div key={camp.id} style={{ border: '1px solid #ddd', borderRadius: '8px', overflow: 'hidden' }}>
                            {editingCampaign?.id !== camp.id ? (
                                <div
                                    onClick={() => setSelectedCampaign(camp)}
                                    style={{ padding: '15px', cursor: 'pointer', background: selectedCampaign?.id === camp.id ? '#e9ecef' : 'white', borderLeft: selectedCampaign?.id === camp.id ? '4px solid #007bff' : '4px solid transparent' }}
                                >
                                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                                        <strong>{camp.name}</strong>
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                setEditingCampaign({
                                                    ...camp,
                                                    start_date: formatForInput(camp.start_date),
                                                    end_date: formatForInput(camp.end_date)
                                                });
                                            }}
                                            style={{ background: '#ffc107', color: '#000', border: 'none', borderRadius: '4px', padding: '2px 8px', fontSize: '0.8em', cursor: 'pointer' }}
                                        >
                                            Edytuj
                                        </button>
                                    </div>
                                    <div style={{ fontSize: '0.8em', color: '#666', marginTop: '5px' }}>{camp.status}</div>
                                </div>
                            ) : (
                                <form onSubmit={handleUpdateCampaign} style={{ display: 'flex', flexDirection: 'column', gap: '10px', padding: '15px', background: '#fff3cd' }}>
                                    <input type="text" required value={editingCampaign.name} onChange={e => setEditingCampaign({...editingCampaign, name: e.target.value})} style={{ padding: '8px' }}/>
                                    <label style={{ fontSize: '0.8em' }}>Nowa data startu:</label>
                                    <input type="datetime-local" required value={editingCampaign.start_date} onChange={e => setEditingCampaign({...editingCampaign, start_date: e.target.value})} style={{ padding: '8px' }}/>
                                    <label style={{ fontSize: '0.8em' }}>Nowa data końca:</label>
                                    <input type="datetime-local" required value={editingCampaign.end_date} onChange={e => setEditingCampaign({...editingCampaign, end_date: e.target.value})} style={{ padding: '8px' }}/>

                                    <div style={{ display: 'flex', gap: '10px' }}>
                                        <button type="submit" style={{ flex: 1, padding: '8px', background: '#28a745', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>Zapisz</button>
                                        <button type="button" onClick={() => setEditingCampaign(null)} style={{ flex: 1, padding: '8px', background: '#6c757d', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>Anuluj</button>
                                    </div>
                                </form>
                            )}
                        </div>
                    ))}
                </div>
            </div>

            <div style={{ flex: 1 }}>
                {!selectedCampaign ? (
                    <div style={{ color: '#888', marginTop: '50px' }}>Wybierz kampanię z listy po lewej stronie, aby załadować dane.</div>
                ) : (
                    <>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <h2>Monitor: {selectedCampaign.name}</h2>
                            <button onClick={() => setShowAdForm(!showAdForm)} style={{ padding: '8px 15px', background: '#17a2b8', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>
                                {showAdForm ? "Anuluj dodawanie" : "+ Dodaj Baner"}
                            </button>
                        </div>

                        {showAdForm && (
                            <form onSubmit={handleCreateAd} style={{ display: 'flex', flexDirection: 'column', gap: '15px', padding: '20px', background: '#f8f9fa', borderRadius: '8px', margin: '20px 0' }}>
                                <div>
                                    <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Plik graficzny (.png, .jpg):</label>
                                    <input type="file" accept="image/*" required onChange={e => setNewAd({...newAd, image: e.target.files[0]})} />
                                </div>
                                <div>
                                    <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Cechy kontekstowe (JSON):</label>
                                    <textarea required rows="3" value={newAd.context_features} onChange={e => setNewAd({...newAd, context_features: e.target.value})} style={{ width: '100%', padding: '8px', fontFamily: 'monospace' }} />
                                    <small style={{ color: '#666' }}>Te dane zostaną przekazane algorytmowi MAB do analizy.</small>
                                </div>
                                <button type="submit" style={{ padding: '10px', background: '#007bff', color: 'white', border: 'none', fontWeight: 'bold' }}>Wgraj reklamę</button>
                            </form>
                        )}

                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(250px, 1fr))', gap: '20px', marginTop: '20px' }}>
                            {ads.length === 0 ? <p>Brak banerów w tej kampanii.</p> : ads.map(ad => {
                                const adStats = stats[ad.id] || { impressions: 0, clicks: 0, ctr: 0 };
                                const isPerformingWell = adStats.ctr > 0.05;

                                return (
                                    <div key={ad.id} style={{ border: '1px solid #eee', borderRadius: '10px', overflow: 'hidden', boxShadow: '0 4px 6px rgba(0,0,0,0.05)' }}>
                                        <div style={{ height: '150px', background: '#f8f9fa', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                                            <img src={ad.image_url} alt="Baner" style={{ maxHeight: '100%', maxWidth: '100%', objectFit: 'contain' }} />
                                        </div>
                                        <div style={{ padding: '15px' }}>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '10px' }}>
                                                <div style={{ fontSize: '0.7em', color: '#888', wordBreak: 'break-all', paddingRight: '10px' }}>ID: {ad.id}</div>
                                                <button
                                                    onClick={() => handleDeleteAd(ad.id)}
                                                    style={{ background: '#dc3545', color: 'white', border: 'none', borderRadius: '4px', padding: '4px 8px', fontSize: '0.8em', cursor: 'pointer', whiteSpace: 'nowrap' }}
                                                >
                                                    Usuń
                                                </button>
                                            </div>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '5px' }}>
                                                <span>Wyświetlenia:</span><strong>{adStats.impressions}</strong>
                                            </div>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '5px' }}>
                                                <span>Kliknięcia:</span><strong>{adStats.clicks}</strong>
                                            </div>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '15px', paddingTop: '15px', borderTop: '1px dashed #ccc', color: isPerformingWell ? '#28a745' : '#000', fontWeight: 'bold' }}>
                                                <span>CTR:</span><span>{(adStats.ctr * 100).toFixed(2)}%</span>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}