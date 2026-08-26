# Getting Started

Waxlight uses Wails v2 to expose Go controller methods to the React/TypeScript frontend. Wails generates the frontend bindings from the controllers bound at application startup; this API reference is generated from the same Go source.

Use the [Methods Index](/api/METHODS) to find a method, then open its controller page for the Go and TypeScript signatures, parameters, return type, errors, example, and source location. DTO pages document the JSON shape used across the bridge.

Generated controller bindings live in `frontend/src/wailsjs/go/wails`, and generated model types live in `frontend/src/wailsjs/go/models.ts`. For example:

```ts
import { ListInstances } from "../wailsjs/go/wails/InstanceController";

const instances = await ListInstances();
```

Every binding returns a `Promise`. Await it inside an async function and handle rejection at the UI boundary. Waxlight's application code normally wraps these bindings in domain modules under `frontend/src/shared/api`, where bridge errors are normalized before reaching components.
