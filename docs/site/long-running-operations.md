# Long-running Operations

Waxlight represents tracked work with `OperationDTO`. A start method such as `StartExistingDataImport` or `GameVersionController.InstallVersion` can return an operation immediately while backend work continues under the application lifecycle.

```text
Frontend start method
        |
        v
   OperationDTO
        |
        | operation:created / updated / progress
        | operation:completed / failed / removed
        v
TanStack Query operations cache
        |
        v
OperationController: ListOperations, CancelOperation, DeleteOperation, ClearOperationHistory
```

The backend persists operation state before publishing most updates. The frontend subscribes to Wails events for immediate cache updates and also refreshes `ListOperations` periodically as a fallback. `OperationController.CancelOperation` requests cancellation by operation ID; deletion and history clearing apply only to finished operations.

Not every tracked operation is an asynchronous, cancellable worker. Snapshot work and some local installs use the same persisted operation model while their controller call remains synchronous. Treat the start method's documented return type and behavior as authoritative rather than assuming every `OperationDTO` can be cancelled.

See [OperationController](/api/controllers/OperationController) for lifecycle controls and [OperationDTO](/api/types/OperationDTO) for states, progress, timestamps, and error fields.
