import { useState, useEffect, useRef } from 'react';
import axios from 'axios';

export default function Store() {
    const [ad, setAd] = useState(null);
    const [loading, setLoading] = useState(true);
    const effectRan = useRef(false);
    const wsRef = useRef(null);
    const shownAdIdRef = useRef(null);

    const publicApi = axios.create({ baseURL: '/api/v1' });

    useEffect(() => {
        if (effectRan.current === true) return;

        const loadStoreAd = async () => {
            try {
                const activeCampaignRes = await publicApi.get('/public/campaigns/active');
                const activeCampaignId = activeCampaignRes.data.id;

                const response = await publicApi.get(`/public/campaigns/${activeCampaignId}/ads`);
                const availableAds = response.data || [];

                if (availableAds.length > 0) {
                    const selectedAd = availableAds[0];
                    setAd(selectedAd);
                    if (shownAdIdRef.current !== selectedAd.id) {
                        shownAdIdRef.current = selectedAd.id;
                        publicApi.post('/events', {
                            ad_id: selectedAd.id,
                            event_type: 'impression',
                            user_context: '{"source": "store", "device": "desktop"}'
                        }).then(() => console.log("Zalogowano wyświetlenie reklamy:", selectedAd.id))
                            .catch(err => console.error("Nie udało się zalogować wyświetlenia:", err));
                    }
                } else {
                    shownAdIdRef.current = null;
                    setAd(null);
                }
            } catch (err) {
                console.error("Brak dostępnych reklam dla sklepu lub błąd konfiguracji publicznej.", err);
                shownAdIdRef.current = null;
                setAd(null);
            } finally {
                setLoading(false);
            }
        };

        loadStoreAd();
        const ws = new WebSocket('ws://localhost:8080/api/v1/ws');
        wsRef.current = ws;

        ws.onopen = () => console.log('Sklep połączony ze strumieniem WebSocket');
        ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            if (msg.type === 'campaign_created' || msg.type === 'ad_created' || msg.type === 'ad_deleted') {
                console.log(`Wykryto zmianę (${msg.type}), odświeżam ofertę sklepu...`);
                loadStoreAd();
            }
        };
        ws.onerror = (err) => console.error('Błąd połączenia WebSocket w sklepie:', err);

        return () => {
            effectRan.current = true;
            if (wsRef.current) wsRef.current.close();
        };
    }, []);
    const handleAdClick = async (e) => {
        if (e && e.preventDefault) e.preventDefault();

        if (!ad) return;
        try {
            await publicApi.post('/events', {
                ad_id: ad.id,
                event_type: 'click',
                user_context: '{"source": "store", "device": "desktop"}'
            });
            console.log("Zalogowano kliknięcie w baner:", ad.id);
            window.alert("Kliknięto w reklamę!");

        } catch (err) {
            console.error("Nie udało się wysłać zdarzenia click:", err);
        }
    };

    return (
        <div style={{ fontFamily: 'sans-serif', background: '#f4f6f9', minHeight: '100vh', padding: '20px' }}>
            <header style={{ background: '#1a1a1a', color: 'white', padding: '15px 30px', borderRadius: '8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h1 style={{ margin: 0, fontSize: '1.5rem' }}>🛒 SuperStore e-Commerce</h1>
                <nav><span style={{ color: '#aaa' }}>Witaj, Gościu</span></nav>
            </header>
            <div style={{ margin: '20px 0', background: 'white', padding: '15px', borderRadius: '8px', boxShadow: '0 2px 4px rgba(0,0,0,0.05)', textAlign: 'center' }}>
                {loading ? (
                    <p style={{ color: '#888' }}>Pobieranie dopasowanej oferty...</p>
                ) : ad ? (
                    <div onClick={handleAdClick} style={{ display: 'block', cursor: 'pointer' }}>
                        <div style={{ maxWidth: '728px', margin: '0 auto', border: '1px solid #ddd', borderRadius: '4px', overflow: 'hidden' }}>
                            <img
                                src={ad.image_url}
                                alt="Sponsorowana oferta"
                                style={{ width: '100%', height: 'auto', display: 'block', maxHeight: '200px', objectFit: 'cover' }}
                            />
                        </div>
                        <small style={{ color: '#aaa', display: 'block', marginTop: '5px' }}>Reklama sponsorowana</small>
                    </div>
                ) : (
                    <p style={{ color: '#888', margin: 0 }}>[Miejsce na Twoją Reklamę]</p>
                )}
            </div>
            <main style={{ marginTop: '30px' }}>
                <h2>Polecane produkty dla Ciebie</h2>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '20px', marginTop: '20px' }}>
                    {[1, 2, 3, 4].map(i => (
                        <div key={i} style={{ background: 'white', padding: '15px', borderRadius: '8px', boxShadow: '0 2px 4px rgba(0,0,0,0.05)' }}>
                            <div style={{ height: '150px', background: '#eee', borderRadius: '4px', marginBottom: '10px' }}></div>
                            <h3>Przykładowy Produkt {i}</h3>
                            <p style={{ color: '#888' }}>Doskonała jakość w inżynierskiej cenie.</p>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '15px' }}>
                                <strong>{99 + i},00 PLN</strong>
                                <button style={{ padding: '6px 12px', background: '#28a745', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>Kup teraz</button>
                            </div>
                        </div>
                    ))}
                </div>
            </main>
        </div>
    );
}