# Privacy Policy

**Last updated: August 9, 2026**

Waxlight Launcher ("Waxlight") is an independent, open-source launcher for Vintage Story maintained by the Waxlight project.

This Privacy Policy explains what information Waxlight processes, what telemetry is sent, why it is used, and how users can disable it.

## 1. Data stored locally

Waxlight stores application data on the user's device, including launcher settings, installed game versions, instances, mods, downloads, and local launcher state.

Vintage Story account credentials are stored using the operating system's native credential store. Waxlight does not intentionally store account passwords or TOTP codes as persistent application data.

Some authentication data may temporarily be required by Vintage Story while the game is running. Waxlight removes temporary authentication values from game configuration as part of its normal cleanup process.

## 2. Telemetry

Waxlight can send limited telemetry to the Waxlight telemetry service.

Telemetry is intended to help understand launcher usage, identify reliability problems, and improve the project.

The telemetry payload may include:

- a randomly generated installation identifier;
- Waxlight version;
- operating system;
- CPU architecture;
- number of configured instances;
- number of installed mods;
- allowlisted launcher lifecycle events, such as creating or deleting an instance, downloading or removing a mod, starting or completing an update, and successful or failed game launches;
- structured error categories, component names, and operation names.

The installation identifier is a randomly generated UUID. It is not derived from hardware identifiers, the operating-system username, file paths, Vintage Story account information, or other personal identifiers.

The telemetry endpoint is:

`https://waxlight.telemetry.amadomuerte.ru`

As with any network request, the server may receive normal connection metadata such as the source IP address while processing a request. Waxlight does not intentionally include the user's IP address in the telemetry JSON payload.

## 3. Data that telemetry does not send

Waxlight telemetry is designed not to send:

- passwords;
- TOTP or other two-factor authentication codes;
- Vintage Story session keys, signatures, tokens, or other authentication secrets;
- email addresses;
- Vintage Story usernames or player names;
- account details;
- instance names;
- mod names;
- local file or directory paths;
- personal files;
- raw logs;
- raw error messages;
- stack traces;
- server response bodies;
- game configuration contents.

Telemetry events and error reports use predefined, allowlisted values rather than arbitrary user-controlled text.

## 4. Disabling telemetry

Telemetry can be disabled from:

**Settings → Privacy & telemetry**

When telemetry is disabled, Waxlight stops sending telemetry events and heartbeats.

The locally generated installation identifier may remain in Waxlight's local settings after telemetry is disabled. Keeping this local value does not itself transmit data.

## 5. Purpose of processing

Telemetry is used for project-related purposes such as:

- measuring overall launcher usage;
- understanding which launcher operations are used;
- detecting broad reliability trends;
- identifying common categories of failures;
- evaluating the effect of updates;
- prioritizing maintenance and development work.

Waxlight does not use telemetry for advertising, behavioral profiling, or selling user data.

## 6. Data sharing

Waxlight does not sell telemetry data to advertisers or data brokers.

Telemetry may be processed by infrastructure providers used to host and operate the Waxlight telemetry service. Those providers may process ordinary network and server metadata as necessary to provide their services.

Waxlight also communicates with third-party services when required for launcher functionality, including Vintage Story services and GitHub. Data processed directly by those third parties is governed by their own privacy policies and is separate from Waxlight telemetry.

## 7. Data retention

Telemetry may be retained for project analytics, debugging, security, and operational purposes.

The Waxlight project does not currently guarantee a fixed retention period. Data should be kept only for as long as reasonably necessary for these purposes, while aggregated or non-identifying statistics may be retained longer.

## 8. User choices and requests

Users can stop future telemetry collection at any time by disabling telemetry in Waxlight settings.

Because telemetry uses a pseudonymous installation identifier rather than a user account, Waxlight may not be able to associate stored telemetry with a specific person unless the relevant installation identifier can be provided.

For privacy questions or requests concerning telemetry data, contact the repository owner through the project's GitHub profile and request a private communication channel. Do not post authentication secrets, installation identifiers, or other sensitive information in a public GitHub issue.

Project repository:

https://github.com/AmadoMuerte/Waxlight-launcher

Repository owner:

https://github.com/AmadoMuerte

## 9. Security

Waxlight is designed to minimize the amount of information included in telemetry and to prevent user-controlled text, credentials, paths, and raw error data from entering telemetry payloads.

Security vulnerabilities or suspected exposure of sensitive data should be reported using the private reporting process described in the project's `docs/SECURITY.md`.

## 10. Open-source transparency

Waxlight is open source. The telemetry implementation can be reviewed in the public source code, including the telemetry models, allowlisted events, identity generation, privacy tests, and HTTP client.

## 11. Changes to this policy

This Privacy Policy may be updated when Waxlight's data practices, telemetry system, infrastructure, or legal requirements change.

Material changes should be reflected in this document and committed to the public Waxlight repository.

## 12. Contact

For privacy-related questions, contact the Waxlight repository owner through:

https://github.com/AmadoMuerte

For security-sensitive matters, use the private vulnerability reporting process described in:

`docs/SECURITY.md`
