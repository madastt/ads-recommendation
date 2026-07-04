import random


class EpsilonGreedyEngine:
    def __init__(self, epsilon=0.1):
        self.epsilon = epsilon

    def choose_ad(self, campaign_id, user_context, available_ads):
        if not available_ads:
            return ""

        if random.random() < self.epsilon:
            print(f"[{campaign_id}] Eksploracja (Epsilon): Losuję 1 z {len(available_ads)} banerów.")
            return random.choice(available_ads)
        else:
            print(f"[{campaign_id}] Eksploatacja: Wybieram wiodącą reklamę.")
            return random.choice(available_ads)