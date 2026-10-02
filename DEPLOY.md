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

## Create Azure Resources

Set unique names and your preferred Azure region. The ACR name must be globally unique and contain only alphanumeric characters.

```powershell
$RG = "waitaminute-prod-rg"
$LOCATION = "centralus"
$ACR = "<globally-unique-acr-name>"
$PLAN = "waitaminute-linux-b1"
$APP = "<globally-unique-webapp-name>"
$IMAGE = "waitaminutedigital-go"

az login
az account set --subscription "<subscription-id>"
az group create --name $RG --location $LOCATION
az acr create --resource-group $RG --name $ACR --sku Basic
```

Build and push an initial image after creating ACR:

```powershell
$LOGIN_SERVER = az acr show --name $ACR --resource-group $RG --query loginServer --output tsv
az acr login --name $ACR
docker tag waitaminutedigital-go:initial "$LOGIN_SERVER/${IMAGE}:initial"
docker push "$LOGIN_SERVER/${IMAGE}:initial"
```

Create the Linux B1 plan and container Web App. The system-assigned identity is used for registry pulls rather than a stored ACR password.

```powershell
az appservice plan create --name $PLAN --resource-group $RG --location $LOCATION --sku B1 --is-linux
az webapp create --name $APP --resource-group $RG --plan $PLAN `
  --container-image-name "$LOGIN_SERVER/${IMAGE}:initial" `
  --assign-identity '[system]' --acr-use-identity --acr-identity '[system]' --https-only true

$PRINCIPAL_ID = az webapp identity show --name $APP --resource-group $RG --query principalId --output tsv
$ACR_ID = az acr show --name $ACR --resource-group $RG --query id --output tsv
az role assignment create --assignee $PRINCIPAL_ID --scope $ACR_ID --role AcrPull
```

Set App Service settings. Generate the bcrypt hash with `go run ./cmd/hashpw`; set a random `SESSION_SECRET` of at least 32 bytes. Keep both values in your shell environment and do not commit them.

```powershell
$env:ADMIN_PASSWORD_HASH = "<bcrypt-hash>"
$env:SESSION_SECRET = "<random-secret-at-least-32-bytes>"

az webapp config appsettings set --name $APP --resource-group $RG --settings `
  APP_ENV=production `
  PORT=8080 `
  SITE_URL="https://${APP}.azurewebsites.net" `
  SITE_DB=/home/data/site.db `
  ADMIN_PASSWORD_HASH="$env:ADMIN_PASSWORD_HASH" `
  SESSION_SECRET="$env:SESSION_SECRET" `
  CLARITY_ID=wp42rt08kj `
  ACS_ENDPOINT="<acs-endpoint>" `
  ACS_ACCESS_KEY="<acs-access-key>" `
  NOTIFY_FROM="<verified-sender>" `
  NOTIFY_TO=corneliustoole@waitaminutedigital.com `
  WEBSITES_PORT=8080 `
  WEBSITES_ENABLE_APP_SERVICE_STORAGE=true
```

`SITE_DB` is persisted at `/home/data/site.db`; uploads are stored at `/home/data/uploads`. The app creates both paths as needed. `BUILD_VERSION` is optional; CI supplies the commit SHA, while an unset value uses the startup content hash.

## Seed Once

The image includes `/app/seed`, built from `cmd/seed`. After the app is running, open an App Service container shell and run it once. It uses the same production app settings and persistent database path. The seed operation is idempotent.

```powershell
az webapp exec --name $APP --resource-group $RG --mode shell --shell /bin/sh
```

At the remote shell prompt:

```sh
/app/seed
exit
```

## GitHub Actions Secrets

Create exactly these repository secrets for `.github/workflows/deploy.yml`:

- `AZURE_CREDENTIALS`: service-principal JSON for an identity with Contributor on the deployment resource group and AcrPush on the registry.
- `AZURE_RESOURCE_GROUP`: resource group containing the Web App and ACR.
- `AZURE_WEBAPP_NAME`: Web App name.
- `ACR_NAME`: ACR resource name, not its login-server URL.

Create a deployment principal and grant its registry role with Azure CLI. Put the returned JSON into the `AZURE_CREDENTIALS` repository secret using your repository's secret-management interface; do not commit it.

```powershell
$RG_ID = az group show --name $RG --query id --output tsv
$ACR_ID = az acr show --name $ACR --resource-group $RG --query id --output tsv
$CREDENTIALS = az ad sp create-for-rbac --name "${APP}-github-deploy" --role Contributor --scopes $RG_ID --json-auth
$CLIENT_ID = ($CREDENTIALS | ConvertFrom-Json).clientId
az role assignment create --assignee $CLIENT_ID --scope $ACR_ID --role AcrPush
```

The Web App identity separately needs AcrPull, as configured above.

## Custom Domain And Free Certificate

First verify the deployed site at `https://${APP}.azurewebsites.net`, including health, public pages, admin login, inquiry submission, uploads, and logs. Do not move the live domain until those checks pass.

Get the Azure values needed by the DNS provider, then create the root-domain A record and validation TXT record at the domain's DNS provider. The domain-provider change is the only step here that cannot be done with Azure CLI unless the DNS zone itself is hosted in Azure DNS.

```powershell
$WEBAPP_IP = az webapp config hostname get-external-ip --resource-group $RG --webapp-name $APP
$DOMAIN_VERIFICATION_ID = az webapp show --name $APP --resource-group $RG --query customDomainVerificationId --output tsv
```

Set `A @` to `$WEBAPP_IP` and `TXT asuid` to `$DOMAIN_VERIFICATION_ID` at the DNS provider. This moves the domain only after the Azure hostname has passed its verification checks. Wait for DNS propagation, then bind the hostname and provision the free App Service managed certificate:

```powershell
az webapp config hostname add --webapp-name $APP --resource-group $RG --hostname waitaminutedigital.com
az webapp config ssl create --name $APP --resource-group $RG --hostname waitaminutedigital.com
$THUMBPRINT = az webapp config ssl list --name $APP --resource-group $RG --query "[?contains(hostNames, 'waitaminutedigital.com')].thumbprint | [0]" --output tsv
az webapp config ssl bind --name $APP --resource-group $RG --hostname waitaminutedigital.com --certificate-thumbprint $THUMBPRINT --ssl-type SNI
az webapp update --name $APP --resource-group $RG --https-only true
az webapp config appsettings set --name $APP --resource-group $RG --settings SITE_URL=https://waitaminutedigital.com
```

The managed-certificate creation command is currently marked Preview by Azure CLI. It requires the custom hostname to be validated and bound first.

## Cutover Checklist

1. Deploy to and verify the `*.azurewebsites.net` hostname first. Confirm the site, health endpoint, admin, contact flow, persisted database, and uploads before moving DNS.
2. Move the domain's A record to the new App Service IP and add/keep the `asuid` TXT verification record. Bind `waitaminutedigital.com`, install the free managed certificate, and set `SITE_URL=https://waitaminutedigital.com`.
3. Verify HTTPS, canonical URLs, sitemap/feed, login, and persisted uploads on the custom domain after DNS propagates.
4. Only after the new domain is verified, delete the old App Service and its old database/storage resource. Confirm each old resource ID before deleting; do not delete the new resource group or the new database.

```powershell
az webapp delete --name "<old-webapp-name>" --resource-group "<old-resource-group>"
az resource show --ids "<old-database-resource-id>" --query "{name:name,type:type,id:id}"
az resource delete --ids "<old-database-resource-id>"
```
