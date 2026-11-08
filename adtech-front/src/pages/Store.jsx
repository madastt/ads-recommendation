
import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import axios from 'axios';

const publicApi = axios.create({ baseURL: '/api/v1' });

const USER_PROFILES = [
    {
        id: 'technology',
        label: 'Technologiczny (18–25 lat)',
        age_group: '18-25',
        interests: ['electronics', 'gaming'],
    },
    {
        id: 'active',
        label: 'Aktywny (26–35 lat)',
        age_group: '26-35',
        interests: ['sport', 'fashion'],
    },
    {
        id: 'reader',
        label: 'Czytelniczy (50+ lat)',
        age_group: '50+',
        interests: ['books'],
    },
];

const getDeviceType = () => {
    const width = window.innerWidth;

    if (width < 768) return 'mobile';
    if (width < 1024) return 'tablet';
    return 'desktop';
};

const DEVICE_LABELS = {
    mobile: 'Telefon',
    tablet: 'Tablet',
    desktop: 'Komputer',
};

export default function Store() {
    const [ad, setAd] = useState(null);
    const [loading, setLoading] = useState(true);

    const [profileId, setProfileId] = useState('technology');
    const [deviceType, setDeviceType] = useState(getDeviceType());
    const [adUserContext, setAdUserContext] = useState('');

    const impressionKeyRef = useRef(null);
    const loadRequestRef = useRef(0);
    const loadStoreAdRef = useRef(null);

    const selectedProfile =
        USER_PROFILES.find((profile) => profile.id === profileId) ||
        USER_PROFILES[0];

    const userContext = useMemo(() => ({
        source: 'store',
        age_group: selectedProfile.age_group,
        interests: selectedProfile.interests,
        device: deviceType,
    }), [selectedProfile, deviceType]);

    const userContextJson = useMemo(
        () => JSON.stringify(userContext),
        [userContext]
    );

    useEffect(() => {
        const handleResize = () => {
            setDeviceType(getDeviceType());
        };

        window.addEventListener('resize', handleResize);

        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    const loadStoreAd = useCallback(async () => {
        const requestId = ++loadRequestRef.current;
        setLoading(true);

        try {
            const activeCampaignRes =
                await publicApi.get('/public/campaigns/active');

            const activeCampaignId = activeCampaignRes.data.id;

            const params = new URLSearchParams();
            params.set('device', userContext.device);
            params.set('age_group', userContext.age_group);

            userContext.interests.forEach((interest) => {
                params.append('interests', interest);
            });

            const response = await publicApi.get(
                `/public/campaigns/${activeCampaignId}/ads?${params.toString()}`
            );

            if (requestId !== loadRequestRef.current) return;

            const availableAds = response.data || [];

            if (availableAds.length > 0) {
                const selectedAd = availableAds[0];

                setAd(selectedAd);
                setAdUserContext(userContextJson);

                const impressionKey =
                    `${selectedAd.id}:${userContextJson}`;

                if (impressionKeyRef.current !== impressionKey) {
                    impressionKeyRef.current = impressionKey;

                    try {
                        await publicApi.post('/events', {
                            ad_id: selectedAd.id,
                            event_type: 'impression',
                            user_context: userContextJson,
                        });

                        console.log(
                            'Zalogowano wyświetlenie reklamy:',
                            selectedAd.id
                        );
                    } catch (err) {
                        if (impressionKeyRef.current === impressionKey) {
                            impressionKeyRef.current = null;
                        }

                        console.error(
                            'Nie udało się zalogować wyświetlenia:',
                            err
                        );
                    }
                }
            } else {
                setAd(null);
                setAdUserContext('');
                impressionKeyRef.current = null;
            }
        } catch (err) {
            if (requestId !== loadRequestRef.current) return;

            console.error(
                'Brak dostępnych reklam lub błąd konfiguracji sklepu.',
                err
            );

            setAd(null);
            setAdUserContext('');
            impressionKeyRef.current = null;
        } finally {
            if (requestId === loadRequestRef.current) {
                setLoading(false);
            }
        }
    }, [userContext, userContextJson]);

    useEffect(() => {
        loadStoreAdRef.current = loadStoreAd;
        loadStoreAd();
    }, [loadStoreAd]);

    useEffect(() => {
        const protocol =
            window.location.protocol === 'https:' ? 'wss:' : 'ws:';

        const ws = new WebSocket(
            `${protocol}//${window.location.host}/api/v1/ws`
        );

        ws.onopen = () => {
            console.log('Sklep połączony ze strumieniem WebSocket');
        };

        ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);

                if (
                    msg.type === 'campaign_created' ||
                    msg.type === 'ad_created' ||
                    msg.type === 'ad_deleted' ||
                    msg.type === 'campaign_archived'
                ) {
                    loadStoreAdRef.current?.();
                }
            } catch (err) {
                console.error('Błąd odczytu WebSocket:', err);
            }
        };

        ws.onerror = (err) => {
            console.error('Błąd WebSocket w sklepie:', err);
        };

        return () => {
            ws.close();
        };
    }, []);

    const handleAdClick = async (e) => {
        if (e?.preventDefault) e.preventDefault();
        if (!ad || !adUserContext) return;

        try {
            await publicApi.post('/events', {
                ad_id: ad.id,
                event_type: 'click',
                user_context: adUserContext,
            });

            console.log('Zalogowano kliknięcie w baner:', ad.id);
            window.alert('Kliknięto w reklamę!');
        } catch (err) {
            console.error('Nie udało się wysłać zdarzenia click:', err);
        }
    };

    return (
        <div style={{
            fontFamily: 'sans-serif',
            background: '#f4f6f9',
            minHeight: '100vh',
            padding: '20px',
        }}>
            <header style={{
                background: '#1a1a1a',
                color: 'white',
                padding: '15px 30px',
                borderRadius: '8px',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
            }}>
                <h1 style={{ margin: 0, fontSize: '1.5rem' }}>
                    🛒 SuperStore e-Commerce
                </h1>
                <nav>
                    <span style={{ color: '#aaa' }}>Witaj, Gościu</span>
                </nav>
            </header>

            <section style={{
                margin: '20px 0',
                padding: '16px',
                background: 'white',
                borderRadius: '8px',
                boxShadow: '0 2px 4px rgba(0,0,0,0.05)',
            }}>
                <label htmlFor="store-profile" style={{
                    display: 'block',
                    marginBottom: '8px',
                    fontWeight: 'bold',
                }}>
                    Profil użytkownika:
                </label>

                <select
                    id="store-profile"
                    value={profileId}
                    onChange={(e) => setProfileId(e.target.value)}
                    style={{
                        padding: '9px 12px',
                        border: '1px solid #ccc',
                        borderRadius: '6px',
                        width: '100%',
                        maxWidth: '360px',
                    }}
                >
                    {USER_PROFILES.map((profile) => (
                        <option key={profile.id} value={profile.id}>
                            {profile.label}
                        </option>
                    ))}
                </select>

                <p style={{ color: '#666', marginBottom: 0 }}>
                    Zainteresowania: {selectedProfile.interests.join(', ')}
                    {' | '}
                    Wykryte urządzenie: {DEVICE_LABELS[deviceType]}
                </p>
            </section>

            <div style={{
                margin: '20px 0',
                background: 'white',
                padding: '15px',
                borderRadius: '8px',
                boxShadow: '0 2px 4px rgba(0,0,0,0.05)',
                textAlign: 'center',
            }}>
                {loading ? (
                    <p style={{ color: '#888' }}>
                        Pobieranie dopasowanej oferty...
                    </p>
                ) : ad ? (
                    <div
                        onClick={handleAdClick}
                        style={{ display: 'block', cursor: 'pointer' }}
                    >
                        <div style={{
                            maxWidth: '728px',
                            height: '200px',
                            margin: '0 auto',
                            overflow: 'hidden',
                            background: '#ffffff',
                        }}>
                            <img
                                src={ad.image_url}
                                alt="Sponsorowana oferta"
                                style={{
                                    width: '100%',
                                    height: '100%',
                                    objectFit: 'contain',
                                    display: 'block',
                                }}
                            />
                        </div>

                        <small style={{
                            color: '#aaa',
                            display: 'block',
                            marginTop: '5px',
                        }}>
                            Reklama sponsorowana
                        </small>
                    </div>
                ) : (
                    <p style={{ color: '#888', margin: 0 }}>
                        [Miejsce na Twoją Reklamę]
                    </p>
                )}
            </div>

            <main style={{ marginTop: '30px' }}>
                <h2>Polecane produkty dla Ciebie</h2>

                <div style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
                    gap: '20px',
                    marginTop: '20px',
                }}>
                    {[1, 2, 3, 4].map((i) => (
                        <div key={i} style={{
                            background: 'white',
                            padding: '15px',
                            borderRadius: '8px',
                            boxShadow: '0 2px 4px rgba(0,0,0,0.05)',
                        }}>
                            <div style={{
                                height: '150px',
                                background: '#eee',
                                borderRadius: '4px',
                                marginBottom: '10px',
                            }} />

                            <h3>Przykładowy Produkt {i}</h3>
                            <p style={{ color: '#888' }}>
                                Doskonała jakość w inżynierskiej cenie.
                            </p>

                            <div style={{
                                display: 'flex',
                                justifyContent: 'space-between',
                                alignItems: 'center',
                                marginTop: '15px',
                            }}>
                                <strong>{99 + i},00 PLN</strong>
                                <button style={{
                                    padding: '6px 12px',
                                    background: '#28a745',
                                    color: 'white',
                                    border: 'none',
                                    borderRadius: '4px',
                                    cursor: 'pointer',
                                }}>
                                    Kup teraz
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            </main>
        </div>
    );
}
