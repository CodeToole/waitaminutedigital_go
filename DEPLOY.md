# Deploy

All Azure resource operations below use Azure CLI. Do not run these commands from this coding session; run them yourself after reviewing the plan values.

## Build For App Service

The laptop is ARM64 and App Service runs this container as amd64. Build the target architecture explicitly so the Go binary and image match Azure. BuildKit cross-compiles the Go binary; Docker Desktop may use QEMU when running the amd64 image locally.

```powershell
docker buildx build --platform linux/amd64 --load -t waitaminutedigital-go:initial .
```

For a local production-like run, provide throwaway values for the required admin hash and session secret, map port 8080, then open `http://localhost:8080/health`:

```powershell
docker run --platform linux/amd64 --rm -p 8080:8080 `
  -e APP_ENV=production `
  -e ADMIN_PASSWORD_HASH="$env:ADMIN_PASSWORD_HASH" `
  -e SESSION_SECRET="$env:SESSION_SECRET" `
  -e SITE_URL="http://localhost:8080" `
  waitaminutedigital-go:initial
```

## Existing Azure Resources

These steps reuse the existing infrastructure in East US 2. They create exactly one Azure resource: the temporary Linux container Web App `waitaminute-go-stage` on the existing Linux B1 plan. They do not create a resource group, plan, registry, database, or replacement for the live app.

```powershell
$SUBSCRIPTION = "Azure subscription 1"
$RG = "WMDpy-Core-RG"
$PLAN = "waitaminute-plan"
$ACR = "wmdacr2026"
$STAGE = "waitaminute-go-stage"
$IMAGE = "waitaminute-go"

az login
az account set --subscription $SUBSCRIPTION
$LOGIN_SERVER = az acr show --name $ACR --resource-group $RG --query loginServer --output tsv
$PLAN_ID = az appservice plan show --name $PLAN --resource-group $RG --query id --output tsv
az acr login --name $ACR
```

Build and push the amd64 image to the existing registry. `BUILD_VERSION` is the source commit SHA and is embedded into the image for versioned asset URLs.

```powershell
$BUILD_VERSION = git rev-parse HEAD
docker buildx build --platform linux/amd64 --build-arg BUILD_VERSION=$BUILD_VERSION --tag "$LOGIN_SERVER/${IMAGE}:$BUILD_VERSION" --push .
```

Create the temporary app on the existing plan using the image above. The app's system-assigned identity pulls from the existing ACR; it does not need registry credentials stored in app settings.

```powershell
az webapp create --name $STAGE --resource-group $RG --plan $PLAN_ID `
  --container-image-name "$LOGIN_SERVER/${IMAGE}:$BUILD_VERSION" `
  --assign-identity '[system]' --acr-use-identity --acr-identity '[system]' --https-only true

$STAGE_PRINCIPAL_ID = az webapp identity show --name $STAGE --resource-group $RG --query principalId --output tsv
$ACR_ID = az acr show --name $ACR --resource-group $RG --query id --output tsv
az role assignment create --assignee $STAGE_PRINCIPAL_ID --scope $ACR_ID --role AcrPull
```

Generate the bcrypt hash with `go run ./cmd/hashpw`; choose a `SESSION_SECRET` with at least 32 random bytes. Set those and the ACS values securely, then configure the stage app. Initially `SITE_URL` is the stage app's `azurewebsites.net` address. In production, the app stores SQLite at `/home/data/site.db` and uploads at `/home/data/uploads`; `WEBSITES_ENABLE_APP_SERVICE_STORAGE=true` persists both under `/home`.

```powershell
$env:ADMIN_PASSWORD_HASH = "<bcrypt-hash>"
$env:SESSION_SECRET = "<random-secret-at-least-32-bytes>"
$env:ACS_ENDPOINT = "<acs-endpoint>"
$env:ACS_ACCESS_KEY = "<acs-access-key>"
$env:NOTIFY_FROM = "<verified-sender>"
$env:NOTIFY_TO = "corneliustoole@waitaminutedigital.com"
$STAGE_URL = "https://${STAGE}.azurewebsites.net"

az webapp config appsettings set --name $STAGE --resource-group $RG --settings `
  APP_ENV=production `
  PORT=8080 `
  WEBSITES_PORT=8080 `
  WEBSITES_ENABLE_APP_SERVICE_STORAGE=true `
  SITE_DB=/home/data/site.db `
  SITE_URL=$STAGE_URL `
  BUILD_VERSION=$BUILD_VERSION `
  ADMIN_PASSWORD_HASH="$env:ADMIN_PASSWORD_HASH" `
  SESSION_SECRET="$env:SESSION_SECRET" `
  CLARITY_ID=wp42rt08kj `
  ACS_ENDPOINT="$env:ACS_ENDPOINT" `
  ACS_ACCESS_KEY="$env:ACS_ACCESS_KEY" `
  NOTIFY_FROM="$env:NOTIFY_FROM" `
  NOTIFY_TO="$env:NOTIFY_TO"
```

## Seed Once And Verify

After `waitaminute-go-stage` starts successfully, run `/app/seed` exactly once against its persistent database. Do not add seeding to startup or the deployment workflow. The seed command is idempotent, but the production procedure is a single explicit execution.

```powershell
az webapp exec --name $STAGE --resource-group $RG --mode shell --shell /bin/sh
```

At the container prompt, run it once and exit:

```sh
/app/seed
exit
```

Before changing the domain, verify `https://${STAGE}.azurewebsites.net`:

- `/health` returns HTTP 200 and `{"ok":true}`.
- Home, dispatch listing, seeded article, Game Room, Projects, About, and Contact render.
- Admin login works; create/publish a test dispatch and verify its public article page.
- Upload a supported image and verify it remains available after an app restart.
- Submit a valid contact inquiry; confirm it appears under Admin > Inquiries and the ACS notification arrives with the visitor as Reply-To.
- `/sitemap.xml` is valid XML and lists the published seeded dispatch; `/robots.txt` disallows `/admin` and links the sitemap; `/feed.xml` is valid RSS and contains published dispatches only.
- Check container startup and application logs, then remove test content if desired.

Use the web app's app settings for the admin hash and ACS credentials; do not put real secret values in source control or command transcripts.

## GitHub Actions Configuration

Create exactly these repository secrets for `.github/workflows/deploy.yml`:

- `AZURE_CREDENTIALS`: service-principal JSON for an identity with Contributor on `WMDpy-Core-RG` and `AcrPush` on `wmdacr2026`.
- `ADMIN_PASSWORD_HASH`: bcrypt hash for the production admin password.
- `SESSION_SECRET`: at least 32 random bytes for production session storage.
- `ACS_ENDPOINT`: Azure Communication Services endpoint.
- `ACS_ACCESS_KEY`: Azure Communication Services access key.
- `NOTIFY_FROM`: verified ACS sender address.
- `NOTIFY_TO`: inquiry notification recipient (normally `corneliustoole@waitaminutedigital.com`).

Create the repository variable `SITE_URL` as `https://waitaminute-go-stage.azurewebsites.net` initially. During cutover, change it to `https://waitaminutedigital.com`; it is a variable, not a secret, and the workflow applies its current value.

```powershell
$RG_ID = az group show --name WMDpy-Core-RG --query id --output tsv
$ACR_ID = az acr show --name wmdacr2026 --resource-group WMDpy-Core-RG --query id --output tsv
$CREDENTIALS = az ad sp create-for-rbac --name "waitaminute-go-deploy" --role Contributor --scopes $RG_ID --json-auth
$CLIENT_ID = ($CREDENTIALS | ConvertFrom-Json).clientId
az role assignment create --assignee $CLIENT_ID --scope $ACR_ID --role AcrPush
```

Store the returned JSON in the `AZURE_CREDENTIALS` repository secret using the repository secret-management interface; do not commit it. The stage Web App's own system-assigned identity separately needs `AcrPull`, as configured in the existing-resource setup above.

## Domain Cutover And Free Certificate

Do not start cutover until every item in the stage verification checklist passes. The live app remains `waitaminute-web-prod` during staging.

Get the new app's external IP and domain-verification ID. DNS is managed at the domain provider unless the domain's DNS zone is already hosted in Azure DNS; do not create a new DNS resource as part of this deployment.

```powershell
$RG = "WMDpy-Core-RG"
$LIVE = "waitaminute-web-prod"
$STAGE = "waitaminute-go-stage"
$WEBAPP_IP = az webapp config hostname get-external-ip --resource-group $RG --webapp-name $STAGE
$DOMAIN_VERIFICATION_ID = az webapp show --name $STAGE --resource-group $RG --query customDomainVerificationId --output tsv
```

First remove `waitaminutedigital.com` from the old live app. Then point the domain provider's root `A @` record at `$WEBAPP_IP` and set `TXT asuid` to `$DOMAIN_VERIFICATION_ID`. Wait for DNS propagation, then bind the hostname to the stage app:

```powershell
az webapp config hostname delete --webapp-name $LIVE --resource-group $RG --hostname waitaminutedigital.com
az webapp config hostname add --webapp-name $STAGE --resource-group $RG --hostname waitaminutedigital.com
```

Request the free Azure App Service managed certificate after the hostname validates, then bind it to the stage app. The managed-certificate creation command is currently marked Preview by Azure CLI.

```powershell
az webapp config ssl create --name $STAGE --resource-group $RG --hostname waitaminutedigital.com
$THUMBPRINT = az webapp config ssl list --name $STAGE --resource-group $RG --query "[?contains(hostNames, 'waitaminutedigital.com')].thumbprint | [0]" --output tsv
az webapp config ssl bind --name $STAGE --resource-group $RG --hostname waitaminutedigital.com --certificate-thumbprint $THUMBPRINT --ssl-type SNI
az webapp update --name $STAGE --resource-group $RG --https-only true
az webapp config appsettings set --name $STAGE --resource-group $RG --settings SITE_URL=https://waitaminutedigital.com
```

Also change the GitHub Actions repository variable `SITE_URL` to `https://waitaminutedigital.com` so future deployments preserve the cutover host.

## Cutover Verification And Cleanup

After DNS and certificate propagation, verify HTTPS and repeat the stage checklist at `https://waitaminutedigital.com`: health, public pages, admin login/publishing, uploads across restart, inquiry persistence and ACS email, sitemap, robots, and RSS. Keep the old resources until all checks pass and you are satisfied the new SQLite database and uploads are correct.

Only after successful verification, remove the old live App Service, the old PostgreSQL Flexible Server, and obsolete certificate resources. List and record the old certificate thumbprint before deleting the app, and confirm that certificate is not used by another app. Do not delete `waitaminute-go-stage`, `waitaminute-plan`, or `wmdacr2026`.

```powershell
az webapp config ssl list --name waitaminute-web-prod --resource-group WMDpy-Core-RG
az webapp delete --name waitaminute-web-prod --resource-group WMDpy-Core-RG
az postgres flexible-server delete --name waitaminuteserverpod --resource-group WMDpy-Core-RG --yes
az webapp config ssl delete --resource-group WMDpy-Core-RG --certificate-thumbprint "<obsolete-certificate-thumbprint>"
```

Azure App Service does not support renaming a Web App resource in place. The app resource can remain named `waitaminute-go-stage`; the public site and custom domain remain `waitaminutedigital.com`.
