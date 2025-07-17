package ehpa

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gocrane/crane/pkg/metrics"
	"github.com/gocrane/crane/pkg/utils"
)

type hpaEventHandler struct {
	enqueueHandler handler.EnqueueRequestForObject
}

func (h *hpaEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	pod := evt.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if pod.DeletionTimestamp != nil {
		h.Delete(ctx, event.DeleteEvent{Object: evt.Object}, q)
		return
	}

	h.enqueueHandler.Create(ctx, evt, q)
}

func (h *hpaEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.enqueueHandler.Delete(ctx, evt, q)
}

func (h *hpaEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	newHpa := evt.ObjectNew.(*autoscalingv2.HorizontalPodAutoscaler)
	oldHpa := evt.ObjectOld.(*autoscalingv2.HorizontalPodAutoscaler)
	klog.V(6).Infof("hpa %s OnUpdate", klog.KObj(newHpa))
	if oldHpa.Status.DesiredReplicas != newHpa.Status.DesiredReplicas {
		for _, cond := range newHpa.Status.Conditions {
			if cond.Reason == "SucceededRescale" || cond.Reason == "SucceededOverloadRescale" {
				scaleType := "hpa"
				if utils.IsHPAControlledByEHPA(newHpa) {
					scaleType = "ehpa"
				}

				labels := map[string]string{
					"namespace": newHpa.Namespace,
					"name":      newHpa.Name,
					"type":      scaleType,
				}
				metrics.HPAScaleCount.With(labels).Inc()

				break
			}
		}
	}
	h.enqueueHandler.Update(ctx, evt, q)
}

func (h *hpaEventHandler) Generic(evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}
