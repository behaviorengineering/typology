# billing (develop)

<!-- typology:generated -->

Billing bounded context for fixture tests.

Human map follows Typology (slice → owns → subprograms → surfaces). DocPageKind files are leaves.

```text
Slice
├── Overview          [overview.md](overview.md)
├── Owns              [components.md](components.md)
├── Subprograms
│   └── [invoice](subprograms/invoice.md)
├── Actuators
│   └── [invoice-webhook](actuators/invoice-webhook.md)
└── Surfaces
    ├── API           [contracts.md](contracts.md)
    └── Jobs           [pipelines.md](pipelines.md)
```
