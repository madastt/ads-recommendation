import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    vus: 50,          // 50 uzytkownikow
    duration: '30s',
};

export default function () {
    const campaignId = '12d32def-0af2-404b-b565-c8d34efacc4a';

    const res = http.get(`http://api:8080/api/v1/public/campaigns/${campaignId}/ads`);

    check(res, {
        'status is 200': (r) => r.status === 200,
        'latency is under 100ms': (r) => r.timings.duration < 100,
    });

    sleep(0.5);
}