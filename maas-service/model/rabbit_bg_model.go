package model

import (
	"fmt"
	"github.com/lib/pq"
	"time"
)

type BgStatus struct {
	Id         int            `json:"-"`
	Namespace  string         `json:"namespace" example:"namespace"`
	Timestamp  time.Time      `json:"timestamp" example:"2021-08-18T17:29:06.82837Z"`
	Active     string         `json:"active" example:"v2"`
	Legacy     string         `json:"legacy" example:"v3"`
	Candidates pq.StringArray `gorm:"type:string[]" json:"candidates" example:"v2,v3"`
}

func (cpMessage BgStatus) GetAllVersions() []string {
	var versions []string
	versions = append(versions, cpMessage.Active)
	versions = append(versions, cpMessage.Legacy)
	versions = append(versions, cpMessage.Candidates...)

	return versions
}

type CpMessageDto []CpDeploymentVersion

type CpDeploymentVersion struct {
	Version     string `json:"version" example:"v1"`
	Stage       string `json:"stage" example:"ACTIVE"`
	CreatedWhen string `json:"createdWhen" example:"2021-08-18T16:33:15.142354459Z"`
	UpdatedWhen string `json:"updatedWhen" example:"2021-08-18T16:33:15.14235456Z"`
}

func (CpMessageDto CpMessageDto) ConvertToBgStatus() (*BgStatus, error) {
	var cpMessage *BgStatus
	cpMessage = new(BgStatus)
	for _, state := range CpMessageDto {
		if state.Stage == "ACTIVE" {
			cpMessage.Active = state.Version
		}
		if state.Stage == "LEGACY" {
			cpMessage.Legacy = state.Version
		}
		if state.Stage == "CANDIDATE" {
			cpMessage.Candidates = append(cpMessage.Candidates, state.Version)
		}
	}

	activeVersion, err := ParseVersion(cpMessage.Active)
	if err != nil {
		return nil, fmt.Errorf("incorrect active version format: %w", err)
	}
	if activeVersion == "" {
		return nil, fmt.Errorf("active version couldn't be empty")
	}

	_, err = ParseVersion(cpMessage.Legacy)
	if err != nil {
		return nil, fmt.Errorf("incorrect legacy version format: %w", err)
	}

	for _, candVersion := range cpMessage.Candidates {
		_, err = ParseVersion(candVersion)
		if err != nil {
			return nil, fmt.Errorf("incorrect legacy version format: %w", err)
		}
	}

	return cpMessage, nil
}
