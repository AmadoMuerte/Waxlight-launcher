package wails

import (
	"context"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/statistics"
)

type statisticsQueries interface {
	Overview(context.Context) (statistics.Statistics, error)
}

// StatisticsController exposes aggregated launcher statistics to the
// frontend. It stays limited to DTO conversion and feature invocation.
type StatisticsController struct {
	svc       statisticsQueries
	lifecycle lifecycle
}

func NewStatisticsController(service statisticsQueries, lifecycle lifecycle) *StatisticsController {
	return &StatisticsController{svc: service, lifecycle: lifecycle}
}

func (controller *StatisticsController) GetOverviewStatistics() (
	StatisticsDTO,
	error,
) {
	statistics, err := controller.svc.Overview(controller.lifecycle.Context())
	return statisticsDTO(statistics), err
}
