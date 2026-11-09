import random
import json
import threading
import numpy as np


class LinUCBEngine:
    AGE_GROUPS = ["18-25", "26-35", "36-50", "50+"]
    DEVICES = ["desktop", "mobile", "tablet"]
    INTERESTS = [
        "electronics",
        "gaming",
        "fashion",
        "books",
        "sport",
    ]

    def __init__(self, alpha=0.5, warm_start_weight=0.2):
        self.alpha = alpha
        self.warm_start_weight = warm_start_weight

        self.d = 1 + len(self.AGE_GROUPS) + len(self.DEVICES) + len(self.INTERESTS)

        self.A = {}
        self.b = {}
        self.warm_started_ads = set()
        self.lock = threading.Lock()

    def _ensure_ad_initialized(self, ad_id, ad_context_json=None):
        if ad_id not in self.A:
            self.A[ad_id] = np.identity(self.d, dtype=np.float64)
            self.b[ad_id] = np.zeros((self.d, 1), dtype=np.float64)

        if ad_context_json and ad_id not in self.warm_started_ads:
            self._apply_warm_start(ad_id, ad_context_json)
            self.warm_started_ads.add(ad_id)

    def _apply_warm_start(self, ad_id, ad_context_json):
        x_init = self._context_to_vector(ad_context_json, include_bias=False)
        self.b[ad_id] += self.warm_start_weight * x_init
        print(f"[LinUCB] Nałożono warm-start dla reklamy {ad_id[:8]}...")

    def _context_to_vector(self, context_str, include_bias=True):
        vec = np.zeros(self.d, dtype=np.float64)
        if include_bias:
            vec[0] = 1.0

        if not context_str:
            return vec.reshape((self.d, 1))

        try:
            data = json.loads(context_str) if isinstance(context_str, str) else context_str
        except (json.JSONDecodeError, TypeError):
            return vec.reshape((self.d, 1))

        offset = 1

        age_val = (
            data.get("age_group")
            or data.get("age")
            or data.get("target_age")
        )
        if isinstance(age_val, str):
            age_val = [age_val]
        if isinstance(age_val, list):
            for i, age_group in enumerate(self.AGE_GROUPS):
                if age_group in age_val:
                    vec[offset + i] = 1.0
        offset += len(self.AGE_GROUPS)

        device_val = data.get("device") or data.get("user_device")
        if isinstance(device_val, str):
            device_val = [device_val]
        if isinstance(device_val, list):
            for i, dev in enumerate(self.DEVICES):
                if dev in device_val:
                    vec[offset + i] = 1.0
        offset += len(self.DEVICES)

        interests_val = data.get("interests") or data.get("user_interests") or []
        if isinstance(interests_val, str):
            interests_val = [interests_val]
        if isinstance(interests_val, list):
            for i, interest in enumerate(self.INTERESTS):
                if interest in interests_val:
                    vec[offset + i] = 1.0

        return vec.reshape((self.d, 1))

    def choose_ad(self, campaign_id, user_context, available_ads):
        if not available_ads:
            return ""

        x_t = self._context_to_vector(user_context, include_bias=True)

        with self.lock:
            best_ads = []
            max_ucb = -float("inf")

            for ad_id in available_ads:
                self._ensure_ad_initialized(ad_id)

                A_a = self.A[ad_id]
                b_a = self.b[ad_id]

                theta_a = np.linalg.solve(A_a, b_a)
                A_inv_x = np.linalg.solve(A_a, x_t)

                expected_reward = float(np.dot(theta_a.T, x_t)[0, 0])
                uncertainty = self.alpha * float(np.sqrt(np.dot(x_t.T, A_inv_x)[0, 0]))
                ucb_score = expected_reward + uncertainty

                if np.isclose(ucb_score, max_ucb, rtol=1e-7, atol=1e-9):
                    best_ads.append(ad_id)
                elif ucb_score > max_ucb:
                    max_ucb = ucb_score
                    best_ads = [ad_id]

            selected_ad = random.choice(best_ads)
            print(f"[{campaign_id}] LinUCB: Wybrano {selected_ad[:8]}... "
                  f"(UCB: {max_ucb:.4f}, remisujących: {len(best_ads)})")
            return selected_ad
    def _apply_event_update(self, ad_id, event_type, user_context):
        x_t = self._context_to_vector(user_context, include_bias=True)

        if event_type == "impression":
            self.A[ad_id] += np.dot(x_t, x_t.T)
            return True

        if event_type == "click":
            self.b[ad_id] += x_t
            return True

        return False

    def update_stats(self, ad_id, event_type, user_context=""):
        if event_type not in ("impression", "click"):
            return

        with self.lock:
            self._ensure_ad_initialized(ad_id)

            updated = self._apply_event_update(
                ad_id,
                event_type,
                user_context
            )

        if updated:
            print(
                f"[LinUCB] Zaktualizowano model dla reklamy "
                f"{ad_id[:8]}... ({event_type})"
            )

    def hydrate(
            self,
            ad_contexts_map=None,
            historical_events=None,
            replace_all=False
    ):
        if ad_contexts_map is None:
            ad_contexts_map = {}

        if historical_events is None:
            historical_events = []

        with self.lock:

            if replace_all:
                self.A.clear()
                self.b.clear()
                self.warm_started_ads.clear()

            # Inicjalizacja wszystkich reklam i zastosowanie warm-startu.
            for ad_id, ad_context in ad_contexts_map.items():
                self._ensure_ad_initialized(
                    ad_id,
                    ad_context
                )

            restored_events = 0

            # Odtworzenie historycznych aktualizacji modelu.
            for event in historical_events:
                ad_id = event.get("ad_id")
                event_type = event.get("event_type")
                user_context = event.get("user_context") or "{}"

                if not ad_id:
                    continue

                if event_type not in ("impression", "click"):
                    continue

                self._ensure_ad_initialized(
                    ad_id,
                    ad_contexts_map.get(ad_id)
                )

                if self._apply_event_update(
                        ad_id,
                        event_type,
                        user_context
                ):
                    restored_events += 1

            print(
                f"[LinUCB] Synchronizacja zakończona. "
                f"Reklamy: {len(self.A)}, "
                f"odtworzone zdarzenia: {restored_events}"
            )