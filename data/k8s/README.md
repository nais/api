# K8s data

This directory contains data for faking resources.

`namespace` and `labels.team` are automatically added to each resource based on the directory structure.

## Structure

```
├── cluster-name
│   └── namespace
│       ├── resource-1.yaml
│       └── resource-2.yaml
└── README.md
```

Each resource may contains multiple resources, separated by `---` on a line.
