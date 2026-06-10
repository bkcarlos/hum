#!/usr/bin/env bash
# Deploy Hum to Google Cloud Run (single-service: one Go process serves the SPA
# + /api, built from the repo-root Dockerfile via Cloud Build).
#
# Prereqs: gcloud CLI installed & logged in (`gcloud auth login`), billing enabled.
#
# Usage:
#   GCP_PROJECT=my-project \
#   APPLE_TEAM_ID=ABCDE12345 APPLE_KEY_ID=KEY1234567 \
#   P8_FILE=./AuthKey_KEY1234567.p8 \
#   [GCP_REGION=asia-northeast1] [SERVICE=hum] [MIN_INSTANCES=0] \
#   ./deploy/cloudrun.sh
set -euo pipefail

PROJECT="${GCP_PROJECT:?set GCP_PROJECT (your GCP project id)}"
REGION="${GCP_REGION:-asia-northeast1}"
SERVICE="${SERVICE:-hum}"
APPLE_TEAM_ID="${APPLE_TEAM_ID:?set APPLE_TEAM_ID}"
APPLE_KEY_ID="${APPLE_KEY_ID:?set APPLE_KEY_ID}"
P8_FILE="${P8_FILE:?set P8_FILE (path to your AuthKey_*.p8)}"
SECRET="${SECRET:-apple-p8}"
MIN_INSTANCES="${MIN_INSTANCES:-0}"   # set to 1 to eliminate cold starts

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

echo "▸ project=$PROJECT region=$REGION service=$SERVICE min-instances=$MIN_INSTANCES"
gcloud config set project "$PROJECT" >/dev/null

echo "▸ enabling APIs (idempotent)…"
gcloud services enable \
  run.googleapis.com cloudbuild.googleapis.com \
  artifactregistry.googleapis.com secretmanager.googleapis.com >/dev/null

echo "▸ storing .p8 in Secret Manager as '$SECRET'…"
if gcloud secrets describe "$SECRET" >/dev/null 2>&1; then
  gcloud secrets versions add "$SECRET" --data-file="$P8_FILE" >/dev/null
else
  gcloud secrets create "$SECRET" --data-file="$P8_FILE" --replication-policy=automatic >/dev/null
fi

PROJNUM="$(gcloud projects describe "$PROJECT" --format='value(projectNumber)')"
RUNTIME_SA="${PROJNUM}-compute@developer.gserviceaccount.com"
echo "▸ granting $RUNTIME_SA read access to the secret…"
gcloud secrets add-iam-policy-binding "$SECRET" \
  --member="serviceAccount:${RUNTIME_SA}" \
  --role="roles/secretmanager.secretAccessor" >/dev/null

echo "▸ building + deploying (Cloud Build uses ./Dockerfile)…"
gcloud run deploy "$SERVICE" \
  --source . \
  --region "$REGION" \
  --platform managed \
  --allow-unauthenticated \
  --port 8080 \
  --cpu 1 --memory 512Mi \
  --min-instances "$MIN_INSTANCES" --max-instances 3 \
  --set-env-vars "APPLE_TEAM_ID=${APPLE_TEAM_ID},APPLE_KEY_ID=${APPLE_KEY_ID},GIN_MODE=release,TRUSTED_PROXY_HOPS=1" \
  --set-secrets "APPLE_PRIVATE_KEY=${SECRET}:latest"

URL="$(gcloud run services describe "$SERVICE" --region "$REGION" --format='value(status.url)')"
echo
echo "✓ deployed: $URL"
echo "  health:   $URL/api/health"
