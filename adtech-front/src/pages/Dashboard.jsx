import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiClient } from '../api/client';
import './Dashboard.css';

const getErrorMessage = (error) => {
    const data = error?.response?.data;
    if (typeof data === 'string') return data;
    if (data?.message) return data.message;
    if (data?.error) return data.error;
    return error?.message || 'Nieznany błąd.';
};

const formatForInput = (isoString) => {
    if (!isoString) return '';
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) return '';
    return new Date(date.getTime() - date.getTimezoneOffset() * 60000)
        .toISOString()
        .slice(0, 16);
};

const toISOStringOrThrow = (value) => {
    const date = new Date(value);
    if (!value || Number.isNaN(date.getTime())) {
        throw new Error('Podaj poprawną datę.');
    }
    return date.toISOString();
};

const validateCampaignDates = (startDate, endDate) => {
    const start = new Date(startDate);
    const end = new Date(endDate);

    if (
        !startDate || !endDate ||
        Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())
    ) {
        throw new Error('Podaj poprawne daty rozpoczęcia i zakończenia kampanii.');
    }
    if (end <= start) {
        throw new Error('Data zakończenia musi być późniejsza niż data rozpoczęcia.');
    }
};

export default function Dashboard() {
    const [campaigns, setCampaigns] = useState([]);
    const [selectedCampaign, setSelectedCampaign] = useState(null);
    const [ads, setAds] = useState([]);
    const [stats, setStats] = useState({});
    const [campaignsLoading, setCampaignsLoading] = useState(true);
    const [campaignsError, setCampaignsError] = useState('');
    const [showArchivedCampaigns, setShowArchivedCampaigns] = useState(false);
    const [detailsError, setDetailsError] = useState('');
    const [brokenImages, setBrokenImages] = useState({});

    const navigate = useNavigate();
    const wsRef = useRef(null);
    const fileInputRef = useRef(null);

    const [showCampaignForm, setShowCampaignForm] = useState(false);
    const [newCampaign, setNewCampaign] = useState({
        name: '', start_date: '', end_date: '',
    });
    const [showAdForm, setShowAdForm] = useState(false);
    const [newAd, setNewAd] = useState({
        context_features: '{"target_age": "18-25", "device": "desktop"}',
        image: null,
    });
    const [editingCampaign, setEditingCampaign] = useState(null);

    const selectedCampaignId = selectedCampaign?.id;
    const selectedCampaignArchived = selectedCampaign?.status === 'archived';
    const maxCtr = Math.max(0, ...ads.map((ad) => Number(stats[ad.id]?.ctr) || 0));
    const activeCampaigns = campaigns.filter((campaign) => campaign.status !== 'archived');
    const archivedCampaigns = campaigns.filter((campaign) => campaign.status === 'archived');

    const handleLogout = useCallback(() => {
        localStorage.removeItem('jwt_token');
        navigate('/login');
    }, [navigate]);

    const fetchCampaigns = useCallback(async () => {
        setCampaignsLoading(true);
        setCampaignsError('');

        try {
            const response = await apiClient.get('/campaigns');
            setCampaigns(response.data || []);
        } catch (error) {
            if (error.response?.status === 401) {
                handleLogout();
                return;
            }
            console.error('Błąd pobierania kampanii:', error);
            setCampaignsError('Nie udało się pobrać kampanii. Sprawdź połączenie z API.');
        } finally {
            setCampaignsLoading(false);
        }
    }, [handleLogout]);

    useEffect(() => {
        fetchCampaigns();
    }, [fetchCampaigns]);

    useEffect(() => {
        if (!selectedCampaignId) {
            setAds([]);
            setStats({});
            setDetailsError('');
            return undefined;
        }

        let isCurrent = true;
        setAds([]);
        setStats({});
        setDetailsError('');
        setShowAdForm(false);

        const fetchDetails = async () => {
            try {
                const [adsResponse, statsResponse] = await Promise.all([
                    apiClient.get(`/campaigns/${selectedCampaignId}/ads`),
                    apiClient.get(`/campaigns/${selectedCampaignId}/stats`),
                ]);
                if (!isCurrent) return;

                setAds(adsResponse.data || []);
                const statsMap = {};
                (statsResponse.data || []).forEach((item) => {
                    statsMap[item.ad_id] = {
                        ...item,
                        impressions: Number(item.impressions) || 0,
                        clicks: Number(item.clicks) || 0,
                        ctr: Number(item.ctr) || 0,
                    };
                });
                setStats(statsMap);
            } catch (error) {
                if (!isCurrent) return;
                console.error('Błąd pobierania szczegółów kampanii:', error);
                setDetailsError('Nie udało się pobrać reklam lub statystyk tej kampanii.');
            }
        };

        fetchDetails();

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const websocket = new WebSocket(
            `${protocol}//${window.location.host}/api/v1/ws`
        );
        wsRef.current = websocket;

        websocket.onopen = () => console.log('Połączono ze strumieniem WebSocket');
        websocket.onerror = (error) => console.error('Błąd połączenia WebSocket:', error);
        websocket.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type !== 'event' || !message.payload) return;

                const newEvent = message.payload;
                const adId = newEvent.ad_id;
                if (!adId) return;
                if (
                    newEvent.campaign_id &&
                    String(newEvent.campaign_id) !== String(selectedCampaignId)
                ) return;

                setStats((previousStats) => {
                    const current = previousStats[adId] || {
                        impressions: 0, clicks: 0, ctr: 0,
                    };
                    const impressions = Number(current.impressions || 0) +
                        (newEvent.event_type === 'impression' ? 1 : 0);
                    const clicks = Number(current.clicks || 0) +
                        (newEvent.event_type === 'click' ? 1 : 0);

                    return {
                        ...previousStats,
                        [adId]: {
                            ...current,
                            impressions,
                            clicks,
                            ctr: impressions > 0 ? clicks / impressions : 0,
                        },
                    };
                });
            } catch (error) {
                console.error('Nie udało się odczytać komunikatu WebSocket:', error);
            }
        };

        return () => {
            isCurrent = false;
            websocket.onmessage = null;
            websocket.close();
            if (wsRef.current === websocket) wsRef.current = null;
        };
    }, [selectedCampaignId]);

    const handleCreateCampaign = async (event) => {
        event.preventDefault();
        try {
            validateCampaignDates(newCampaign.start_date, newCampaign.end_date);
            const payload = {
                name: newCampaign.name.trim(),
                start_date: toISOStringOrThrow(newCampaign.start_date),
                end_date: toISOStringOrThrow(newCampaign.end_date),
            };
            if (!payload.name) throw new Error('Nazwa kampanii nie może być pusta.');

            await apiClient.post('/campaigns', payload);
            await fetchCampaigns();
            setShowCampaignForm(false);
            setNewCampaign({ name: '', start_date: '', end_date: '' });
        } catch (error) {
            alert(`Nie udało się utworzyć kampanii: ${getErrorMessage(error)}`);
        }
    };

    const handleUpdateCampaign = async (event) => {
        event.preventDefault();
        if (!editingCampaign) return;

        try {
            validateCampaignDates(editingCampaign.start_date, editingCampaign.end_date);
            const payload = {
                name: editingCampaign.name.trim(),
                start_date: toISOStringOrThrow(editingCampaign.start_date),
                end_date: toISOStringOrThrow(editingCampaign.end_date),
            };
            if (!payload.name) throw new Error('Nazwa kampanii nie może być pusta.');

            await apiClient.put(`/campaigns/${editingCampaign.id}`, payload);
            await fetchCampaigns();
            setSelectedCampaign((previous) =>
                previous?.id === editingCampaign.id ? { ...previous, ...payload } : previous
            );
            setEditingCampaign(null);
        } catch (error) {
            alert(`Nie udało się zaktualizować kampanii: ${getErrorMessage(error)}`);
        }
    };

    const handleCreateAd = async (event) => {
        event.preventDefault();
        if (!selectedCampaign) return alert('Wybierz kampanię z listy.');
        if (selectedCampaignArchived) {
            return alert('Nie można dodawać reklam do zarchiwizowanej kampanii.');
        }
        if (!newAd.image) return alert('Wybierz plik graficzny.');

        try {
            const features = JSON.parse(newAd.context_features);
            if (features === null || Array.isArray(features) || typeof features !== 'object') {
                throw new Error('Cechy kontekstowe muszą być obiektem JSON.');
            }

            const formData = new FormData();
            formData.append('campaign_id', selectedCampaign.id);
            formData.append('context_features', JSON.stringify(features));
            formData.append('image', newAd.image);
            await apiClient.post('/ads', formData, {
                headers: {
                    'Content-Type': 'multipart/form-data',
                }
            });

            const response = await apiClient.get(`/campaigns/${selectedCampaign.id}/ads`);
            setAds(response.data || []);
            setBrokenImages({});
            setShowAdForm(false);
            setNewAd({ context_features: newAd.context_features, image: null });
            if (fileInputRef.current) fileInputRef.current.value = '';
        } catch (error) {
            alert(`Nie udało się dodać reklamy: ${getErrorMessage(error)}`);
        }
    };

    const handleDeleteAd = async (adId) => {
        const confirmed = window.confirm(
            'Czy na pewno chcesz usunąć ten baner? Spowoduje to również usunięcie jego statystyk.'
        );
        if (!confirmed) return;

        try {
            await apiClient.delete(`/ads/${adId}`);
            setAds((previous) => previous.filter((ad) => ad.id !== adId));
            setStats((previous) => {
                const updated = { ...previous };
                delete updated[adId];
                return updated;
            });
            setBrokenImages((previous) => {
                const updated = { ...previous };
                delete updated[adId];
                return updated;
            });
        } catch (error) {
            alert(`Nie udało się usunąć reklamy: ${getErrorMessage(error)}`);
        }
    };

    const handleArchiveCampaign = async (campaignId, event) => {
        event.stopPropagation();
        const confirmed = window.confirm(
            'Czy na pewno chcesz zarchiwizować tę kampanię? Zostanie wyłączona, ale zachowa swoje statystyki.'
        );
        if (!confirmed) return;

        try {
            await apiClient.delete(`/campaigns/${campaignId}`);
            setCampaigns((previous) => previous.map((campaign) =>
                campaign.id === campaignId ? { ...campaign, status: 'archived' } : campaign
            ));
            setSelectedCampaign((previous) =>
                previous?.id === campaignId ? { ...previous, status: 'archived' } : previous
            );
            if (selectedCampaignId === campaignId) setShowAdForm(false);
        } catch (error) {
            alert(`Nie udało się zarchiwizować kampanii: ${getErrorMessage(error)}`);
        }
    };

    const renderCampaign = (campaign) => {
        const isSelected = selectedCampaign?.id === campaign.id;
        const isArchived = campaign.status === 'archived';
        const isEditing = editingCampaign?.id === campaign.id;

        return (
            <div className="campaign-card" key={campaign.id}>
                {isEditing ? (
                    <form className="form-card campaign-edit-form" onSubmit={handleUpdateCampaign}>
                        <div>
                            <label htmlFor={`campaign-name-${campaign.id}`}>Nazwa kampanii</label>
                            <input
                                id={`campaign-name-${campaign.id}`} type="text" required
                                value={editingCampaign.name}
                                onChange={(event) => setEditingCampaign({ ...editingCampaign, name: event.target.value })}
                            />
                        </div>
                        <div>
                            <label htmlFor={`campaign-start-${campaign.id}`}>Data rozpoczęcia</label>
                            <input
                                id={`campaign-start-${campaign.id}`} type="datetime-local" required
                                value={editingCampaign.start_date}
                                onChange={(event) => setEditingCampaign({ ...editingCampaign, start_date: event.target.value })}
                            />
                        </div>
                        <div>
                            <label htmlFor={`campaign-end-${campaign.id}`}>Data zakończenia</label>
                            <input
                                id={`campaign-end-${campaign.id}`} type="datetime-local" required
                                value={editingCampaign.end_date}
                                onChange={(event) => setEditingCampaign({ ...editingCampaign, end_date: event.target.value })}
                            />
                        </div>
                        <button type="submit" className="btn-submit">Zapisz zmiany</button>
                        <button type="button" className="btn-secondary" onClick={() => setEditingCampaign(null)}>
                            Anuluj
                        </button>
                    </form>
                ) : (
                    <div
                        className={`campaign-item ${isSelected ? 'is-selected' : ''} ${isArchived ? 'is-archived-item' : ''}`}
                        onClick={() => setSelectedCampaign(campaign)}
                    >
                        <div className="campaign-item-header">
                            <strong className="campaign-name">{campaign.name}</strong>
                            <div className="campaign-actions">
                                {!isArchived && (
                                    <>
                                        <button
                                            type="button" className="btn-secondary btn-small"
                                            onClick={(event) => {
                                                event.stopPropagation();
                                                setEditingCampaign({
                                                    ...campaign,
                                                    start_date: formatForInput(campaign.start_date),
                                                    end_date: formatForInput(campaign.end_date),
                                                });
                                            }}
                                        >Edytuj</button>
                                        <button
                                            type="button" className="btn-secondary btn-small"
                                            onClick={(event) => handleArchiveCampaign(campaign.id, event)}
                                        >Archiwizuj</button>
                                    </>
                                )}
                            </div>
                        </div>
                        <div className={`campaign-status ${isArchived ? 'is-archived' : 'is-active'}`}>
                            {(campaign.status || 'active').toUpperCase()}
                        </div>
                    </div>
                )}
            </div>
        );
    };

    return (
        <div className="dashboard">
            <aside className="dashboard-sidebar">
                <div className="sidebar-header">
                    <h2>Kampanie</h2>
                    <button type="button" className="btn-danger btn-logout" onClick={handleLogout}>
                        Wyloguj
                    </button>
                </div>

                <section className="sidebar-navigation">
                    <h3>Nawigacja systemowa</h3>
                    <div className="sidebar-nav-list">
                        <button type="button" className="btn-nav" onClick={() => navigate('/admin/stats')}>
                            📊 Obserwowalność API
                        </button>
                        <button
                            type="button"
                            className="btn-nav btn-nav-store"
                            onClick={() => window.open('/store', '_blank', 'noopener,noreferrer')}
                        >
                            🛒 Otwórz sklep
                        </button>
                    </div>
                </section>

                <button
                    type="button"
                    className="btn-primary btn-full"
                    onClick={() => setShowCampaignForm((visible) => !visible)}
                >
                    {showCampaignForm ? 'Anuluj' : '+ Nowa kampania'}
                </button>

                {showCampaignForm && (
                    <form className="form-card" onSubmit={handleCreateCampaign}>
                        <div>
                            <label htmlFor="new-campaign-name">Nazwa kampanii</label>
                            <input
                                id="new-campaign-name" type="text" required
                                placeholder="Np. Kampania letnia" value={newCampaign.name}
                                onChange={(event) => setNewCampaign({ ...newCampaign, name: event.target.value })}
                            />
                        </div>
                        <div>
                            <label htmlFor="new-campaign-start">Data rozpoczęcia</label>
                            <input
                                id="new-campaign-start" type="datetime-local" required
                                value={newCampaign.start_date}
                                onChange={(event) => setNewCampaign({ ...newCampaign, start_date: event.target.value })}
                            />
                        </div>
                        <div>
                            <label htmlFor="new-campaign-end">Data zakończenia</label>
                            <input
                                id="new-campaign-end" type="datetime-local" required
                                value={newCampaign.end_date}
                                onChange={(event) => setNewCampaign({ ...newCampaign, end_date: event.target.value })}
                            />
                        </div>
                        <button type="submit" className="btn-submit">Utwórz kampanię</button>
                    </form>
                )}

                <div className="campaign-list">
                    {campaignsLoading && <p className="empty-state">Ładowanie kampanii...</p>}
                    {!campaignsLoading && campaignsError && (
                        <p className="empty-state" role="alert">{campaignsError}</p>
                    )}
                    {!campaignsLoading && !campaignsError && campaigns.length === 0 && (
                        <p className="empty-state">Nie ma jeszcze żadnych kampanii.</p>
                    )}
                    {!campaignsLoading && !campaignsError && campaigns.length > 0 && activeCampaigns.length === 0 && (
                        <p className="empty-state">Brak aktywnych kampanii.</p>
                    )}

                    {activeCampaigns.map(renderCampaign)}

                    {archivedCampaigns.length > 0 && (
                        <section className="archived-campaigns">
                            <button
                                type="button"
                                className="archived-toggle"
                                aria-expanded={showArchivedCampaigns}
                                onClick={() => setShowArchivedCampaigns((visible) => !visible)}
                            >
                                <span className="archived-toggle-label">
                                    <span className="archived-chevron" aria-hidden="true">
                                        {showArchivedCampaigns ? '▾' : '▸'}
                                    </span>
                                    Zarchiwizowane kampanie
                                </span>
                                <span className="archived-count">{archivedCampaigns.length}</span>
                            </button>
                            {showArchivedCampaigns && (
                                <div className="archived-campaign-list">
                                    {archivedCampaigns.map(renderCampaign)}
                                </div>
                            )}
                        </section>
                    )}
                </div>
            </aside>

            <main className="dashboard-main">
                {!selectedCampaign ? (
                    <div className="empty-state">
                        Wybierz kampanię z listy, aby wyświetlić jej reklamy i statystyki.
                    </div>
                ) : (
                    <>
                        <div className="main-header">
                            <div>
                                <h2>{selectedCampaign.name}</h2>
                                {selectedCampaignArchived && (
                                    <p className="campaign-status is-archived">
                                        Kampania jest zarchiwizowana i nie przyjmuje nowych reklam.
                                    </p>
                                )}
                            </div>
                            <button
                                type="button" className="btn-add-ad"
                                disabled={selectedCampaignArchived}
                                onClick={() => setShowAdForm((visible) => !visible)}
                            >
                                {showAdForm ? 'Anuluj dodawanie' : '+ Dodaj reklamę'}
                            </button>
                        </div>

                        {showAdForm && !selectedCampaignArchived && (
                            <form className="form-card" onSubmit={handleCreateAd}>
                                <div>
                                    <label htmlFor="ad-image">Plik graficzny (.png, .jpg)</label>
                                    <input
                                        ref={fileInputRef} id="ad-image" type="file"
                                        accept="image/png,image/jpeg,image/jpg,image/webp" required
                                        onChange={(event) => setNewAd({
                                            ...newAd, image: event.target.files?.[0] || null,
                                        })}
                                    />
                                </div>
                                <div>
                                    <label htmlFor="ad-context-features">Cechy kontekstowe (JSON)</label>
                                    <textarea
                                        id="ad-context-features" required rows={4}
                                        value={newAd.context_features}
                                        onChange={(event) => setNewAd({
                                            ...newAd, context_features: event.target.value,
                                        })}
                                    />
                                    <small>
                                        Cechy kontekstowe zostaną przekazane algorytmowi MAB.
                                        Wprowadź obiekt JSON, np. z cechami urządzenia lub grupy wiekowej.
                                    </small>
                                </div>
                                <button type="submit" className="btn-submit">Wgraj reklamę</button>
                            </form>
                        )}

                        {detailsError && <p className="empty-state" role="alert">{detailsError}</p>}

                        <div className="ads-grid">
                            {ads.length === 0 ? (
                                <p className="empty-ads">Brak reklam w tej kampanii.</p>
                            ) : ads.map((ad) => {
                                const adStats = stats[ad.id] || { impressions: 0, clicks: 0, ctr: 0 };
                                const ctr = Number(adStats.ctr) || 0;
                                const isHighestCtr = maxCtr > 0 && Math.abs(ctr - maxCtr) < 1e-9;
                                const imageBroken = brokenImages[ad.id];

                                return (
                                    <article className="ad-card" key={ad.id}>
                                        <div className="ad-preview">
                                            {ad.image_url && !imageBroken ? (
                                                <img
                                                    className="ad-image" src={ad.image_url}
                                                    alt={`Podgląd reklamy ${ad.id}`} loading="lazy"
                                                    onError={() => setBrokenImages((previous) => ({
                                                        ...previous, [ad.id]: true,
                                                    }))}
                                                />
                                            ) : (
                                                <span className="ad-preview-placeholder">Brak podglądu reklamy</span>
                                            )}
                                        </div>
                                        <div className="ad-details">
                                            <div className="ad-meta-row">
                                                <div className="ad-id">ID: {ad.id}</div>
                                                <button
                                                    type="button" className="btn-danger btn-small"
                                                    onClick={() => handleDeleteAd(ad.id)}
                                                >Usuń</button>
                                            </div>
                                            <div className="ad-stat-row">
                                                <span>Wyświetlenia</span>
                                                <strong>{Number(adStats.impressions) || 0}</strong>
                                            </div>
                                            <div className="ad-stat-row">
                                                <span>Kliknięcia</span>
                                                <strong>{Number(adStats.clicks) || 0}</strong>
                                            </div>
                                            <div className={`ad-ctr-row ${isHighestCtr ? 'is-highest-ctr' : ''}`}>
                                                <span>CTR</span>
                                                <span>{(ctr * 100).toFixed(2)}%</span>
                                            </div>
                                        </div>
                                    </article>
                                );
                            })}
                        </div>
                    </>
                )}
            </main>
        </div>
    );
}
