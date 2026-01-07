package checker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/unleash"
	"github.com/sirupsen/logrus"
)

const unleashEnvironment = "production"

type Unleash struct {
	UnleashWatcher *watcher.Watcher[*unleash.UnleashInstance]
	BifrostClient  unleash.BifrostClient
	Log            logrus.FieldLogger
}

func (u Unleash) Run(ctx context.Context) ([]Issue, error) {
	if u.UnleashWatcher == nil || !u.UnleashWatcher.Enabled() {
		u.Log.Debug("unleash watcher not enabled, skipping unleash issue check")
		return nil, nil
	}

	// Get release channels to determine current major version
	channels, err := u.BifrostClient.ListChannels(ctx)
	if err != nil {
		u.Log.WithError(err).Error("failed to get release channels")
		return nil, err
	}

	if channels.JSON200 == nil || len(*channels.JSON200) == 0 {
		u.Log.Debug("no release channels found, skipping unleash issue check")
		return nil, nil
	}

	// Build a map of channel name to channel info, and find current major version
	channelMap := make(map[string]channelInfo)
	currentMajorVersion := 0

	for _, ch := range *channels.JSON200 {
		majorVersion, err := parseMajorVersion(ch.CurrentVersion)
		if err != nil {
			u.Log.WithError(err).WithField("channel", ch.Name).WithField("version", ch.CurrentVersion).Warn("failed to parse channel version")
			continue
		}

		channelMap[ch.Name] = channelInfo{
			name:         ch.Name,
			majorVersion: majorVersion,
		}

		if majorVersion > currentMajorVersion {
			currentMajorVersion = majorVersion
		}
	}

	if currentMajorVersion == 0 {
		u.Log.Warn("could not determine current major version from release channels")
		return nil, nil
	}

	u.Log.WithField("currentMajorVersion", currentMajorVersion).Debug("determined current major version from release channels")

	ret := make([]Issue, 0)

	for _, instance := range u.UnleashWatcher.All() {
		channelName := instance.Obj.ReleaseChannelName()

		// Check for missing release channel
		if channelName == nil || *channelName == "" {
			ret = append(ret, Issue{
				ResourceName: instance.Obj.Name,
				ResourceType: issue.ResourceTypeUnleash,
				Env:          unleashEnvironment,
				Team:         instance.Obj.TeamSlug.String(),
				IssueType:    issue.IssueTypeUnleashMissingReleaseChannel,
				Message:      "Unleash instance is not configured with a release channel",
				Severity:     issue.SeverityCritical,
			})
			continue
		}

		// Look up the channel info
		chInfo, found := channelMap[*channelName]
		if !found {
			u.Log.WithField("channel", *channelName).WithField("instance", instance.Obj.Name).Warn("instance is on unknown release channel")
			// Treat unknown channel as missing channel
			ret = append(ret, Issue{
				ResourceName: instance.Obj.Name,
				ResourceType: issue.ResourceTypeUnleash,
				Env:          unleashEnvironment,
				Team:         instance.Obj.TeamSlug.String(),
				IssueType:    issue.IssueTypeUnleashMissingReleaseChannel,
				Message:      fmt.Sprintf("Unleash instance is on unknown release channel: %s", *channelName),
				Severity:     issue.SeverityCritical,
			})
			continue
		}

		// Check if the channel's major version is outdated
		if chInfo.majorVersion >= currentMajorVersion {
			// Instance is on current version, no issue
			continue
		}

		var severity issue.Severity
		if chInfo.majorVersion < currentMajorVersion-1 {
			// More than one major version behind (e.g., 5.x when 7.x is current)
			severity = issue.SeverityCritical
		} else {
			// One major version behind (e.g., 6.x when 7.x is current)
			severity = issue.SeverityWarning
		}

		ret = append(ret, Issue{
			ResourceName: instance.Obj.Name,
			ResourceType: issue.ResourceTypeUnleash,
			Env:          unleashEnvironment,
			Team:         instance.Obj.TeamSlug.String(),
			IssueType:    issue.IssueTypeUnleashReleaseChannel,
			Message:      fmt.Sprintf("Unleash instance is on release channel '%s' (version %d.x), current version is %d.x", chInfo.name, chInfo.majorVersion, currentMajorVersion),
			Severity:     severity,
			IssueDetails: issue.UnleashReleaseChannelIssueDetails{
				ChannelName:         chInfo.name,
				MajorVersion:        chInfo.majorVersion,
				CurrentMajorVersion: currentMajorVersion,
			},
		})
	}

	return ret, nil
}

type channelInfo struct {
	name         string
	majorVersion int
}

// parseMajorVersion extracts the major version number from a version string like "7.1.0" or "5.12.0-beta.1"
func parseMajorVersion(version string) (int, error) {
	if version == "" {
		return 0, fmt.Errorf("empty version string")
	}

	// Split on "." and take the first part
	parts := strings.Split(version, ".")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}

	majorStr := parts[0]
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse major version from %s: %w", version, err)
	}

	return major, nil
}
