import sys
import os
sys.path.append(os.path.join(os.path.dirname(__file__), 'pb'))
import grpc
from concurrent import futures
import mab_pb2
import mab_pb2_grpc
from engine import LinUCBEngine


class MabService(mab_pb2_grpc.MabEngineServicer):
    def __init__(self):
        self.engine = LinUCBEngine(alpha=0.5, warm_start_weight=0.2)
        print("Silnik LinUCB podłączony.")

    def GetNextAd(self, request, context):
        print(f"\nOtrzymano żądanie z serwera Go | Kampania: {request.campaign_id}")
        selected_ad = self.engine.choose_ad(
            campaign_id=request.campaign_id,
            user_context=request.user_context,
            available_ads=list(request.available_ad_ids)
        )

        print(f"Zwracam do Reacta reklamę ID: {selected_ad}")
        return mab_pb2.DecisionResponse(selected_ad_id=selected_ad)

    def RecordEvent(self, request, context):
        self.engine.update_stats(
            ad_id=request.ad_id,
            event_type=request.event_type,
            user_context=request.user_context
        )
        return mab_pb2.EventResponse(success=True)

    def SyncState(self, request, context):
        self.engine.hydrate(
            impressions_map=request.impressions,
            clicks_map=request.clicks,
            ad_contexts_map=request.ad_contexts
        )
        return mab_pb2.SyncResponse(
            success=True,
            message="Pamięć algorytmu LinUCB została pomyślnie zsynchronizowana"
        )


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    mab_pb2_grpc.add_MabEngineServicer_to_server(MabService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()

    print("Mikroserwis MAB uruchomiony. Nasłuchuję na porcie 50051...")
    server.wait_for_termination()


if __name__ == '__main__':
    serve()