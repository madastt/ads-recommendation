import random

class EpsilonGreedyEngine:
    def __init__(self, epsilon=0.15):
        self.epsilon = epsilon
        self.impressions = {}
        self.clicks = {}

    def update_stats(self, ad_id, event_type):
        if ad_id not in self.impressions:
            self.impressions[ad_id] = 0
            self.clicks[ad_id] = 0

        if event_type == "impression":
            self.impressions[ad_id] += 1
            print(f"Zarejestrowano wyświetlenie dla {ad_id[:8]}... (Łącznie: {self.impressions[ad_id]})")
        elif event_type == "click":
            self.clicks[ad_id] += 1
            print(f"Zarejestrowano KLIKNIĘCIE dla {ad_id[:8]}! (Łącznie: {self.clicks[ad_id]})")

    def hydrate(self, impressions_map, clicks_map):
        self.impressions = dict(impressions_map)
        self.clicks = dict(clicks_map)
        print(f"Zsynchronizowano stan! Reklamy w pamięci: {len(self.impressions)}")

    def choose_ad(self, campaign_id, user_context, available_ads):
        if not available_ads:
            return ""

        for ad in available_ads:
            if ad not in self.impressions:
                self.impressions[ad] = 0
                self.clicks[ad] = 0

        if random.random() < self.epsilon:
            print(f"[{campaign_id}]Eksploracja (Epsilon): Losuję baner.")
            return random.choice(available_ads)
        best_ad = available_ads[0]
        max_ctr = -1.0

        for ad in available_ads:
            ctr = (self.clicks[ad] / self.impressions[ad]) if self.impressions[ad] > 0 else 0.0
            if ctr > max_ctr:
                max_ctr = ctr
                best_ad = ad

        print(f"[{campaign_id}]Eksploatacja: Wybieram lidera {best_ad[:8]}... (CTR: {max_ctr:.2%})")
        return best_ad