package presentation

import (
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type InstanceSnapshotDTO struct {
	ID           string            `json:"id"`
	InstanceID   string            `json:"instanceId"`
	InstanceName string            `json:"instanceName"`
	Type         string            `json:"type"`
	Reason       string            `json:"reason,omitempty"`
	Context      map[string]string `json:"context,omitempty"`
	GameVersion  string            `json:"gameVersion"`
	CreatedAt    string            `json:"createdAt"`
	SizeBytes    int64             `json:"sizeBytes"`
	ModCount     int               `json:"modCount"`
	WorldCount   int               `json:"worldCount"`
}

func instanceSnapshotDTO(snapshot domain.InstanceSnapshot) InstanceSnapshotDTO {
	return InstanceSnapshotDTO{
		ID:           snapshot.ID,
		InstanceID:   snapshot.InstanceID,
		InstanceName: snapshot.InstanceName,
		Type:         string(snapshot.Type),
		Reason:       string(snapshot.Reason),
		Context:      snapshot.Context,
		GameVersion:  snapshot.GameVersion,
		CreatedAt:    iso(snapshot.CreatedAt),
		SizeBytes:    snapshot.SizeBytes,
		ModCount:     snapshot.ModCount,
		WorldCount:   snapshot.WorldCount,
	}
}
