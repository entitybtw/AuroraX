# Security Policy

  ## Reporting a Vulnerability

  The Aurora team takes security issues seriously. We appreciate your efforts to responsibly
   disclose any vulnerabilities you find.

  **Please do NOT report security vulnerabilities through public GitHub issues.**

  Instead, please report them via one of the following methods:

  1. **GitHub Security Advisories (preferred)**: Use [GitHub's private vulnerability
  reporting](https://github.com/aurorallm/aurora/security/advisories/new) to submit a report
  directly through the repository.
  2. **Email**: Send an email to **team.auroragate@gmail.com** with the details of the
  vulnerability.

  ### What to include

  To help us triage and respond quickly, please include:

  - A description of the vulnerability and its potential impact
  - Step-by-step instructions to reproduce the issue
  - Affected version(s) and component(s)
  - Any relevant configuration or environment details
  - Proof-of-concept code, if available

  ### What to expect

  - **Acknowledgment**: We will acknowledge receipt of your report within **48 hours**.
  - **Updates**: We will provide status updates as we investigate, typically within **5
  business days**.
  - **Resolution**: Once a fix is available, we will coordinate with you on disclosure
  timing.
  - **Credit**: We are happy to credit reporters in our release notes and security advisories
   (unless you prefer to remain anonymous).

  ## Supported Versions

  Security updates are provided for the latest release. We recommend always running the most
  recent version.

  ## Security Considerations

  Aurora is an AI gateway that routes requests to multiple LLM providers. When deploying
  Aurora, keep the following in mind:

  - **API Key Management**: Aurora handles provider API keys. Ensure keys are stored
  securely and never committed to version control. Use environment variables or a secrets
  manager.
  - **Network Exposure**: Restrict access to the Aurora admin interface and API endpoints
  using firewalls, VPNs, or authentication layers appropriate for your environment.
  - **TLS**: Always use TLS when exposing Aurora to external networks.
  - **Least Privilege**: Use Aurora's API key management features to enforce least-privilege
  access to upstream providers.
  - **Configuration**: Review your `config.yaml` and `.env` files for any sensitive values
  before sharing or committing them.

  ## OSS Boundary

  This OSS tree provides gateway routing, analytics, and provider operations. Report any path
  that lets a user enable restricted runtime behavior by changing config, env vars, license
  fields, dashboard overrides, build tags, or dropped-in files. Treat those as
  security-sensitive boundary bypasses.

  ## Disclosure Policy

  We follow a coordinated disclosure process:

  1. The reporter submits the vulnerability privately.
  2. We confirm the issue and develop a fix.
  3. We release the fix and publish a security advisory.
  4. The vulnerability details are made public after users have had reasonable time to update
   (typically 30 days after the fix is released).

  We kindly ask that you do not publicly disclose the vulnerability until we have had a
  chance to address it.

  ## Scope

  The following are **in scope** for security reports:

  - The Aurora Gateway (core, internal packages, apps)
  - The Aurora Dashboard UI
  - The Aurora Docker image (`aurorahq/aurora`)
  - The Aurora CLI tools

  The following are **out of scope**:

  - Social engineering attacks
  - Denial of service attacks that rely purely on volumetric traffic
  - Physical security attacks
