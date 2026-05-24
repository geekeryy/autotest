#!/usr/bin/env bash
# One-time setup: GitHub Actions → Firebase Hosting via Workload Identity Federation (no JSON keys).
#
# Usage:
#   GITHUB_REPO=geekeryy/autotest ./scripts/setup-firebase-github-wif.sh
#
# Requires: gcloud, roles/iam.workloadIdentityPoolAdmin (or Owner) on the Firebase/GCP project.

set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-geekeryy}"
GITHUB_REPO="${GITHUB_REPO:-geekeryy/autotest}"
POOL_ID="${WIF_POOL_ID:-github-actions}"
PROVIDER_ID="${WIF_PROVIDER_ID:-github}"
SA_NAME="${WIF_SA_NAME:-github-firebase-deploy}"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

PROJECT_NUMBER="$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
WIF_PROVIDER="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/providers/${PROVIDER_ID}"

echo "Project:        ${PROJECT_ID} (${PROJECT_NUMBER})"
echo "GitHub repo:    ${GITHUB_REPO}"
echo "Service account: ${SA_EMAIL}"
echo

if ! gcloud iam workload-identity-pools describe "${POOL_ID}" \
  --project="${PROJECT_ID}" --location=global >/dev/null 2>&1; then
  echo "Creating workload identity pool ${POOL_ID}..."
  gcloud iam workload-identity-pools create "${POOL_ID}" \
    --project="${PROJECT_ID}" \
    --location=global \
    --display-name="GitHub Actions"
else
  echo "Workload identity pool ${POOL_ID} already exists."
fi

if ! gcloud iam workload-identity-pools providers describe "${PROVIDER_ID}" \
  --project="${PROJECT_ID}" --location=global \
  --workload-identity-pool="${POOL_ID}" >/dev/null 2>&1; then
  echo "Creating OIDC provider ${PROVIDER_ID}..."
  gcloud iam workload-identity-pools providers create-oidc "${PROVIDER_ID}" \
    --project="${PROJECT_ID}" \
    --location=global \
    --workload-identity-pool="${POOL_ID}" \
    --display-name="GitHub" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner" \
    --attribute-condition="assertion.repository=='${GITHUB_REPO}'"
else
  echo "OIDC provider ${PROVIDER_ID} already exists."
fi

if ! gcloud iam service-accounts describe "${SA_EMAIL}" --project="${PROJECT_ID}" >/dev/null 2>&1; then
  echo "Creating service account ${SA_NAME}..."
  gcloud iam service-accounts create "${SA_NAME}" \
    --project="${PROJECT_ID}" \
    --display-name="GitHub Actions Firebase Hosting deploy"
else
  echo "Service account ${SA_EMAIL} already exists."
fi

echo "Granting Firebase Hosting Admin to ${SA_EMAIL}..."
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/firebasehosting.admin" \
  --condition=None >/dev/null

echo "Allowing ${GITHUB_REPO} to impersonate ${SA_EMAIL}..."
gcloud iam service-accounts add-iam-policy-binding "${SA_EMAIL}" \
  --project="${PROJECT_ID}" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/attribute.repository/${GITHUB_REPO}"

cat <<EOF

Done. Add these GitHub repository variables (Settings → Secrets and variables → Actions → Variables):

  GCP_PROJECT_ID=${PROJECT_ID}
  GCP_WORKLOAD_IDENTITY_PROVIDER=${WIF_PROVIDER}
  GCP_SERVICE_ACCOUNT=${SA_EMAIL}
  VITE_API_BASE_URL=https://your-api-domain.example.com

Then push to main (or run the workflow manually) to deploy.

EOF
