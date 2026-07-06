import React, { useState, useEffect } from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import axios from 'axios';

export default function ApiDashboard() {
    const [stats, setStats] = useState(null);

    useEffect(() => {
        const fetchStats = () => {
            axios.get('/api/v1/admin/stats/api', {
                headers: { Authorization: `Bearer ${localStorage.getItem('jwt_token')}` }
            })
            .then(res => setStats(res.data))
            .catch(err => console.error("Błąd pobierania statystyk:", err));
        };

        fetchStats();
        const interval = setInterval(fetchStats, 5000);
        return () => clearInterval(interval);
    }, []);

    if (!stats) return <div style={{ padding: '20px' }}>Ładowanie logów...</div>;

    return (
        <div style={{ fontFamily: 'sans-serif', padding: '20px', background: '#f4f6f9', minHeight: '100vh' }}>
            <h2 style={{ color: '#333' }}>⚙Obserwowalność Systemu (Observability)</h2>

            <div style={{ display: 'flex', gap: '20px', marginBottom: '30px' }}>
                <div style={{ padding: '20px', background: 'white', borderLeft: '4px solid #3b82f6', borderRadius: '8px', flex: 1, boxShadow: '0 2px 4px rgba(0,0,0,0.05)' }}>
                    <h3 style={{ margin: '0 0 10px 0', color: '#666', fontSize: '1rem' }}>Suma Zapytań</h3>
                    <p style={{ margin: 0, fontSize: '2.5rem', fontWeight: 'bold', color: '#1a1a1a' }}>
                        {stats.total_requests}
                    </p>
                </div>
                <div style={{ padding: '20px', background: 'white', borderLeft: '4px solid #10b981', borderRadius: '8px', flex: 1, boxShadow: '0 2px 4px rgba(0,0,0,0.05)' }}>
                    <h3 style={{ margin: '0 0 10px 0', color: '#666', fontSize: '1rem' }}>Średni Czas Przetwarzania</h3>
                    <p style={{ margin: 0, fontSize: '2.5rem', fontWeight: 'bold', color: stats.avg_latency_ms > 100 ? '#ef4444' : '#1a1a1a' }}>
                        {stats.avg_latency_ms} ms
                    </p>
                </div>
            </div>

            <div style={{ background: 'white', padding: '20px', borderRadius: '8px', boxShadow: '0 2px 4px rgba(0,0,0,0.05)' }}>
                <h3 style={{ marginTop: 0, color: '#333' }}>Najbardziej Obciążone Endpointy</h3>
                <div style={{ height: '300px', width: '100%' }}>
                    <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={stats.top_endpoints} layout="vertical" margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
                            <XAxis type="number" />
                            <YAxis dataKey="path" type="category" width={150} tick={{ fontSize: 12 }} />
                            <Tooltip />
                            <Bar dataKey="calls" fill="#3b82f6" name="Liczba wywołań" radius={[0, 4, 4, 0]} />
                        </BarChart>
                    </ResponsiveContainer>
                </div>
            </div>
        </div>
    );
}