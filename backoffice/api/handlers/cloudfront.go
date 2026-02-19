package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
)

type DistributionInfo struct {
	ID               string    `json:"id"`
	DomainName       string    `json:"domain_name"`
	Status           string    `json:"status"`
	Enabled          bool      `json:"enabled"`
	Comment          string    `json:"comment"`
	PriceClass       string    `json:"price_class"`
	HttpVersion      string    `json:"http_version"`
	Aliases          []string  `json:"aliases"`
	OriginCount      int       `json:"origin_count"`
	CacheBehaviors   int       `json:"cache_behaviors_count"`
	LastModifiedTime time.Time `json:"last_modified_time"`
}

// GetCloudFront returns CloudFront distribution info for this project
func (h *Handler) GetCloudFront(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var distributions []DistributionInfo
	var marker *string

	for {
		out, err := h.cfClient.ListDistributions(ctx, &cloudfront.ListDistributionsInput{
			Marker: marker,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list distributions: "+err.Error())
			return
		}
		if out.DistributionList == nil {
			break
		}
		for _, d := range out.DistributionList.Items {
			info := DistributionInfo{
				OriginCount: len(d.Origins.Items),
			}
			if d.Enabled != nil {
				info.Enabled = *d.Enabled
			}
			if d.Id != nil {
				info.ID = *d.Id
			}
			if d.DomainName != nil {
				info.DomainName = *d.DomainName
			}
			if d.Status != nil {
				info.Status = *d.Status
			}
			if d.Comment != nil {
				info.Comment = *d.Comment
			}
			info.PriceClass = string(d.PriceClass)
			info.HttpVersion = string(d.HttpVersion)
			if d.Aliases != nil {
				for _, a := range d.Aliases.Items {
					info.Aliases = append(info.Aliases, a)
				}
			}
			if d.CacheBehaviors != nil && d.CacheBehaviors.Quantity != nil {
				info.CacheBehaviors = int(*d.CacheBehaviors.Quantity)
			}
			if d.LastModifiedTime != nil {
				info.LastModifiedTime = *d.LastModifiedTime
			}
			distributions = append(distributions, info)
		}
		isTruncated := out.DistributionList.IsTruncated != nil && *out.DistributionList.IsTruncated
		if !isTruncated {
			break
		}
		marker = out.DistributionList.NextMarker
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"distributions": distributions,
		"count":         len(distributions),
	})
}
