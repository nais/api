package gcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Clusters map[string]Cluster

type Cluster struct {
	TeamsFolderID int64
	ProjectID     string
}

func (c *Clusters) Decode(value string) error {
	*c = make(Clusters)
	if value == "" {
		return nil
	}
	clustersWithStringID := make(map[string]struct {
		TeamsFolderID string `json:"teams_folder_id"`
		ProjectID     string `json:"project_id"`
	})

	err := json.NewDecoder(strings.NewReader(value)).Decode(&clustersWithStringID)
	if err != nil {
		return fmt.Errorf("parse GCP cluster info: %w", err)
	}

	for environment, cluster := range clustersWithStringID {
		folderID, err := strconv.ParseInt(cluster.TeamsFolderID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse GCP cluster info's folder ID: %w", err)
		}
		(*c)[environment] = Cluster{
			TeamsFolderID: folderID,
			ProjectID:     cluster.ProjectID,
		}
	}
	return nil
}
