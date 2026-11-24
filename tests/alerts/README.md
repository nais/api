# Alert rule tests

```bash
mise install
mise run test:alerts
```

The task renders the chart's actual `PrometheusRule` with release name `nais-api`,
checks the rendered rules, and runs each YAML test file in this directory with
`promtool test rules`. It needs no running API, Docker, cluster, or deployment.
It also runs as a prerequisite of `mise run test`, including in CI.

| File                                     | Scenarios                                                                                                                                                                 |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [client_errors.yaml](client_errors.yaml) | Transient errors and counter resets do not page; sustained Hookd or Kubernetes failures page only the affected component after five minutes and resolve when errors stop. |
| [cost.yaml](cost.yaml)                   | Failed runs before the first success, a disappearing success metric, recovery, and the current rule's inability to detect a stalled success counter.                      |
| [usersync.yaml](usersync.yaml)           | Regular successes, failed syncs and recovery, no first success, leader changes with old timestamps on other replicas, and disappearing metrics.                           |

These tests check rule behavior, not whether the application emits the metric
names and scrape labels used by the rules.

## Reading a scenario

Each scenario starts at Unix timestamp zero. `interval` specifies the time
between samples; `evaluation_interval` specifies how often rules are evaluated.
Promtool advances simulated time, so even a 29-hour scenario runs without waiting.

For example, with one sample per minute, `0x59 3600x15` keeps the last successful
usersync at timestamp zero for minutes 0 through 59, then advances it to timestamp
3600 at minute 60. The alert is inactive at minute 45, fires at minute 46, and
resolves at minute 60.

`0+1x8` supplies the counter values `0 1 2 3 4 5 6 7 8`. For the client alerts,
the two-minute `increase` first becomes positive at minute 1. The five-minute
`for` therefore completes at minute 6, not minute 5.

`_` represents a missing sample. `_x60` supplies exactly 60 missing samples,
whereas `0x59` supplies 60 numeric samples. `stale` marks a series that disappeared.

`exp_alerts: []` asserts no firing alert; a pending alert still satisfies it.
Nonempty expectations check the alert labels and annotations as well as whether
it fires. YAML anchors reuse identical expectations within a test file.

To see a failure, change an expectation and rerun `mise run test:alerts`.
Promtool reports the scenario, evaluation time, and expected versus actual alerts.

## Known behavior, not a guarantee of correct alerting

The client rules select `errors_total` with `component="hookd-client"` or
`component="k8s-client"`. The current Hookd and Kubernetes clients use `otelhttp`,
and this repository has no emitter for those legacy series. Unless external
recording rules provide them, these alerts have no matching data. The tests
validate their existing expressions; they do not establish that the series
exist in production.

The usersync rule cannot distinguish a sync that has never succeeded from an
older release that does not expose the metric. Both alert once the target has
45 minutes of scrape history. The no-first-success scenario also reproduces
that rollout behavior.

The cost rule checks `absent_over_time(...[24h])`, not an increase in the success
counter. With no success metric from startup, its four-hour `for` fires at four
hours. If a previously emitted success metric disappears, it fires 28 hours
after the last sample: the 24-hour range plus the four-hour `for`.

More importantly, once the API has successfully updated costs, it continues
exposing that counter even if every subsequent update fails. A constant counter
is still present, so this rule does **not** alert on that failure. The final cost
scenario documents this blind spot explicitly; a passing suite does not mean
the rule detects all stale cost data. The production rules are unchanged.
