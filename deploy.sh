#!/bin/bash
set -e

# ============================================================
# project.sh
# Single entrypoint for project creation and deployment.
#
# New project:  ./project.sh
#   - Creates GitHub repo (skips if already exists)
#   - Adds GitHub Secrets
#   - Creates Postgres database and user (skips if already exists)
#   - Creates project folder structure
#   - Scaffolds Dockerfile and ci.yml
#   - Deploys GitHub Actions runner (skips if already running)
#
# Existing project deploy:  ./project.sh
#   - Bumps patch version tag
#   - Pushes tag to trigger CI/CD pipeline
#   - Waits for pipeline to complete and reports status
#
# Run from the root of your project directory.
# ============================================================

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()   { echo -e "${GREEN}[✓]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }
info()  { echo -e "${BLUE}[→]${NC} $1"; }

# --- Config ---
GITHUB_OWNER="goatcheese82"
APPDATA_BASE="/mnt/user/appdata"
PG_CONTAINER="pg18"
PG_ADMIN_USER="${POSTGRES_ADMIN_USER:-postgres}"
PG_HOST="10.0.0.X"  # Replace with your Unraid IP
POLL_INTERVAL=15
POLL_TIMEOUT=600

# ============================================================
# PREFLIGHT CHECKS
# ============================================================
echo ""
echo "============================================"
echo "  project.sh — goatcheese82 dev stack"
echo "============================================"
echo ""

command -v gh &>/dev/null     || error "GitHub CLI (gh) is not installed."
command -v git &>/dev/null    || error "git is not installed."
command -v docker &>/dev/null || error "Docker is not installed or not in PATH."
command -v psql &>/dev/null   || error "psql is not installed. Run: sudo apt install postgresql-client"
gh auth status &>/dev/null    || error "GitHub CLI is not authenticated. Run: gh auth login"

# SSH auth check
ssh_output=$(ssh -T git@github.com 2>&1 || true)
if ! echo "$ssh_output" | grep -q "successfully authenticated"; then
  error "SSH authentication to GitHub failed. Check your SSH key is loaded and added to GitHub."
fi
log "SSH authentication to GitHub verified."

# ============================================================
# DETECT MODE: new project or existing deploy
# ============================================================
IS_NEW_PROJECT=false
REPO=""

if git rev-parse --git-dir &>/dev/null 2>&1; then
  REMOTE_URL=$(git remote get-url origin 2>/dev/null || true)
  if [[ "$REMOTE_URL" == *"github.com"* ]]; then
    REPO=$(gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null || true)
  fi
fi

if [[ -z "$REPO" ]]; then
  DIR_NAME=$(basename "$PWD")
  if gh repo view "${GITHUB_OWNER}/${DIR_NAME}" &>/dev/null 2>&1; then
    warn "No git remote detected locally, but GitHub repo '${GITHUB_OWNER}/${DIR_NAME}' exists."
    warn "This looks like a partial setup. Attempting to reconnect..."
    git init &>/dev/null || true
    git remote remove origin &>/dev/null || true
    git remote add origin "git@github.com:${GITHUB_OWNER}/${DIR_NAME}.git"
    git fetch origin &>/dev/null
    git branch -M main
    git branch --set-upstream-to=origin/main main &>/dev/null || true
    git reset --hard origin/main &>/dev/null || true
    REPO="${GITHUB_OWNER}/${DIR_NAME}"
    IS_NEW_PROJECT=true
    log "Reconnected to ${REPO}. Continuing setup..."
  else
    IS_NEW_PROJECT=true
    warn "No existing GitHub repository detected. Running in new project mode."
  fi
elif [[ "$REPO" != "" ]]; then
  IS_NEW_PROJECT=false
  log "Existing repository detected: ${REPO}"
fi

# ============================================================
# NEW PROJECT MODE
# ============================================================
if [[ "$IS_NEW_PROJECT" == "true" ]]; then

  echo ""
  echo "--- New Project Setup ---"
  echo ""

  # If reconnected, derive project name from repo
  if [[ -n "$REPO" ]]; then
    PROJECT_NAME=$(basename "$REPO")
    warn "Using project name from reconnected repo: ${PROJECT_NAME}"
    read -p "Confirm project name [${PROJECT_NAME}]: " PROJECT_NAME_INPUT
    PROJECT_NAME="${PROJECT_NAME_INPUT:-$PROJECT_NAME}"
  else
    read -p "Project name (lowercase, no spaces): " PROJECT_NAME
    [[ -z "$PROJECT_NAME" ]] && error "Project name cannot be empty."
  fi

  read -p "Project description: " PROJECT_DESC

  read -p "Private repo? (y/n) [y]: " IS_PRIVATE
  IS_PRIVATE=${IS_PRIVATE:-y}
  [[ "$IS_PRIVATE" == "y" ]] && VISIBILITY="--private" || VISIBILITY="--public"

  read -p "Deploy to secondary server? (y/n) [n]: " IS_SECONDARY
  IS_SECONDARY=${IS_SECONDARY:-n}

  read -p "Postgres DB username: " DB_USER
  [[ -z "$DB_USER" ]] && error "DB username cannot be empty."

  read -s -p "Postgres DB password: " DB_PASSWORD
  echo ""
  [[ -z "$DB_PASSWORD" ]] && error "DB password cannot be empty."

  read -s -p "Postgres admin password (for pg18): " PG_ADMIN_PASSWORD
  echo ""

  # --- Derived vars ---
  DB_NAME="${PROJECT_NAME}"
  REPO_URL="git@github.com:${GITHUB_OWNER}/${PROJECT_NAME}.git"
  REPO_HTTPS_URL="https://github.com/${GITHUB_OWNER}/${PROJECT_NAME}"
  PROJECT_DIR="${APPDATA_BASE}/${PROJECT_NAME}"
  RUNNER_DIR="${APPDATA_BASE}/github-runners/${PROJECT_NAME}"

  echo ""
  echo "--- Summary ---"
  echo "  Repo:        ${REPO_HTTPS_URL}"
  echo "  Visibility:  $( [[ "$IS_PRIVATE" == "y" ]] && echo "Private" || echo "Public" )"
  echo "  Database:    ${DB_NAME}"
  echo "  DB User:     ${DB_USER}"
  echo "  Project dir: $( [[ "$IS_SECONDARY" == "y" ]] && echo "10.0.2.166 via Dockhand" || echo "${PROJECT_DIR}" )"
  echo "  Runner dir:  ${RUNNER_DIR}"
  echo "  Secondary:   $( [[ "$IS_SECONDARY" == "y" ]] && echo "Yes" || echo "No" )"
  echo "---------------"
  read -p "Proceed? (y/n): " CONFIRM
  [[ "$CONFIRM" != "y" ]] && echo "Aborted." && exit 0

  echo ""

  # --- Step 1: Create GitHub repo ---
  info "Checking if GitHub repository already exists..."
  if gh repo view "${GITHUB_OWNER}/${PROJECT_NAME}" &>/dev/null; then
    warn "Repository ${GITHUB_OWNER}/${PROJECT_NAME} already exists — skipping creation."
  else
    info "Creating GitHub repository..."
    gh repo create "${GITHUB_OWNER}/${PROJECT_NAME}" \
      $VISIBILITY \
      --description "${PROJECT_DESC}" \
      --add-readme \
      --gitignore Go \
      && log "Repository created: ${REPO_HTTPS_URL}" \
      || error "Failed to create GitHub repository."
  fi

  # --- Step 2: Clone or reconnect repo locally ---
  if [[ -f ".git/config" ]] && git remote get-url origin &>/dev/null 2>&1; then
    warn "Already inside a connected git repo — skipping clone."
  elif [[ -d "${PROJECT_NAME}/.git" ]]; then
    warn "Directory ./${PROJECT_NAME} already exists with git repo — skipping clone."
    cd "${PROJECT_NAME}"
  elif [[ -d "${PROJECT_NAME}" ]]; then
    warn "Directory ./${PROJECT_NAME} exists but has no git repo — cloning into it."
    cd "${PROJECT_NAME}"
    git clone "${REPO_URL}" .
  else
    info "Cloning repository..."
    git clone "${REPO_URL}" "${PROJECT_NAME}"
    cd "${PROJECT_NAME}"
    log "Repository cloned into ./${PROJECT_NAME}"
  fi

  # Ensure tracking branch is set
  git branch --set-upstream-to=origin/main main &>/dev/null || true

  # --- Step 3: Add GitHub Secrets ---
  info "Adding GitHub Secrets..."
  gh secret set DB_USER     --body "${DB_USER}"     --repo "${GITHUB_OWNER}/${PROJECT_NAME}"
  gh secret set DB_PASSWORD --body "${DB_PASSWORD}" --repo "${GITHUB_OWNER}/${PROJECT_NAME}"
  gh secret set DB_NAME     --body "${DB_NAME}"     --repo "${GITHUB_OWNER}/${PROJECT_NAME}"
  log "GitHub Secrets added."

  # --- Step 4: Create Postgres database and user ---
  info "Creating Postgres database and user..."

  PG_COMMANDS="
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DB_NAME}') THEN
    CREATE DATABASE ${DB_NAME};
    RAISE NOTICE 'Database ${DB_NAME} created.';
  ELSE
    RAISE NOTICE 'Database ${DB_NAME} already exists, skipping.';
  END IF;
END \$\$;

DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DB_USER}') THEN
    CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASSWORD}';
    GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};
    RAISE NOTICE 'User ${DB_USER} created.';
  ELSE
    RAISE NOTICE 'User ${DB_USER} already exists, skipping.';
  END IF;
END \$\$;
"

  if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${PG_CONTAINER}$"; then
    # Running on Unraid — execute directly via docker exec
    echo "$PG_COMMANDS" | docker exec -i ${PG_CONTAINER} psql -U ${PG_ADMIN_USER}
  else
    # Running on dev machine — connect via LAN
    PGPASSWORD="${PG_ADMIN_PASSWORD}" psql \
      -h "${PG_HOST}" \
      -U "${PG_ADMIN_USER}" \
      -d postgres \
      -c "$PG_COMMANDS"
  fi

  log "Postgres database and user verified."

  # --- Step 5: Scaffold Dockerfile ---
  if [[ -f "Dockerfile" ]]; then
    warn "Dockerfile already exists — skipping."
  else
    info "Scaffolding Dockerfile..."
    mkdir -p cmd/server
    printf '%s\n' \
      '# Build stage' \
      'FROM golang:1.24-alpine AS builder' \
      'WORKDIR /app' \
      'COPY go.mod go.sum ./' \
      'RUN go mod download' \
      'COPY . .' \
      'RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server' \
      '' \
      '# Final stage' \
      'FROM alpine:latest' \
      'RUN apk --no-cache add ca-certificates tzdata' \
      'WORKDIR /app' \
      'COPY --from=builder /app/server .' \
      'EXPOSE 8080' \
      'CMD ["./server"]' \
      > Dockerfile
    log "Dockerfile created."
  fi

  # --- Step 6: Scaffold GitHub Actions pipeline ---
  if [[ -f ".github/workflows/ci.yml" ]]; then
    warn ".github/workflows/ci.yml already exists — skipping."
  else
    info "Scaffolding GitHub Actions pipeline..."
    mkdir -p .github/workflows
    printf '%s\n' \
      'name: CI/CD' \
      '' \
      'on:' \
      '  push:' \
      '    branches: [main]' \
      '    tags:' \
      "      - 'v*.*.*'" \
      '' \
      'env:' \
      '  REGISTRY: ghcr.io' \
      '  IMAGE_NAME: ${{ github.repository }}' \
      '' \
      'jobs:' \
      '  test:' \
      '    name: Test' \
      '    runs-on: self-hosted' \
      '' \
      '    services:' \
      '      postgres:' \
      '        image: postgres:18' \
      '        env:' \
      '          POSTGRES_USER: testuser' \
      '          POSTGRES_PASSWORD: testpassword' \
      '          POSTGRES_DB: testdb' \
      '        ports:' \
      '          - 5432:5432' \
      '        options: >-' \
      '          --health-cmd pg_isready' \
      '          --health-interval 10s' \
      '          --health-timeout 5s' \
      '          --health-retries 5' \
      '' \
      '    steps:' \
      '      - name: Checkout' \
      '        uses: actions/checkout@v4' \
      '' \
      '      - name: Set up Go' \
      '        uses: actions/setup-go@v5' \
      '        with:' \
      '          go-version-file: go.mod' \
      '' \
      '      - name: Run tests' \
      '        run: go test ./...' \
      '        env:' \
      '          DATABASE_URL: postgresql://testuser:testpassword@localhost:5432/testdb?sslmode=disable' \
      '' \
      '  build-and-push:' \
      '    name: Build and Push' \
      '    runs-on: self-hosted' \
      '    needs: test' \
      '    if: startsWith(github.ref, '"'"'refs/tags/v'"'"')' \
      '' \
      '    permissions:' \
      '      contents: read' \
      '      packages: write' \
      '' \
      '    steps:' \
      '      - name: Checkout' \
      '        uses: actions/checkout@v4' \
      '' \
      '      - name: Log in to GHCR' \
      '        uses: docker/login-action@v3' \
      '        with:' \
      '          registry: ${{ env.REGISTRY }}' \
      '          username: ${{ github.actor }}' \
      '          password: ${{ secrets.GITHUB_TOKEN }}' \
      '' \
      '      - name: Extract metadata' \
      '        id: meta' \
      '        uses: docker/metadata-action@v5' \
      '        with:' \
      '          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}' \
      '          tags: |' \
      '            type=semver,pattern={{version}}' \
      '            type=semver,pattern={{major}}.{{minor}}' \
      '' \
      '      - name: Build and push' \
      '        uses: docker/build-push-action@v5' \
      '        with:' \
      '          context: .' \
      '          push: true' \
      '          tags: ${{ steps.meta.outputs.tags }}' \
      '          labels: ${{ steps.meta.outputs.labels }}' \
      > .github/workflows/ci.yml
    log "GitHub Actions pipeline scaffolded at .github/workflows/ci.yml"
  fi

  # --- Step 7: Create project folder structure on Unraid ---
  info "Creating project folder structure..."
  if [[ "$IS_SECONDARY" == "y" ]]; then
    warn "Secondary server selected — creating runner dir on Unraid only."
    warn "Create the project folder manually on 10.0.2.166 via Dockhand."
  else
    if [[ -d "${PROJECT_DIR}" ]]; then
      warn "Project folder ${PROJECT_DIR} already exists — skipping."
    else
      if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${PG_CONTAINER}$"; then
        # Running on Unraid — create directly
        mkdir -p "${PROJECT_DIR}"
        printf '%s\n' \
          'services:' \
          '  app:' \
          "    image: ghcr.io/${GITHUB_OWNER}/${PROJECT_NAME}:latest" \
          "    container_name: ${PROJECT_NAME}" \
          '    restart: unless-stopped' \
          '    env_file:' \
          '      - .env' \
          '    ports:' \
          '      - "8080:8080"' \
          '    networks:' \
          '      - dev_network' \
          '' \
          'networks:' \
          '  dev_network:' \
          '    external: true' \
          > "${PROJECT_DIR}/docker-compose.yml"
        printf '%s\n' \
          "DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@pg18:5432/${DB_NAME}?sslmode=disable" \
          > "${PROJECT_DIR}/.env"
        log "Project folder created at ${PROJECT_DIR}."
      else
        warn "Not running on Unraid — skipping remote project folder creation."
        warn "You will need to manually create ${PROJECT_DIR} on Unraid with:"
        warn "  docker-compose.yml using image: ghcr.io/${GITHUB_OWNER}/${PROJECT_NAME}:latest"
        warn "  .env with DATABASE_URL=postgresql://${DB_USER}:***@pg18:5432/${DB_NAME}?sslmode=disable"
      fi
    fi
  fi

  # --- Step 8: Generate runner token and deploy runner ---
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^github-runner-${PROJECT_NAME}$"; then
    warn "Runner container github-runner-${PROJECT_NAME} is already running — skipping."
  else
    info "Generating GitHub Actions runner token..."
    RUNNER_TOKEN=$(gh api \
      --method POST \
      -H "Accept: application/vnd.github+json" \
      /repos/${GITHUB_OWNER}/${PROJECT_NAME}/actions/runners/registration-token \
      --jq '.token')
    [[ -z "$RUNNER_TOKEN" ]] && error "Failed to generate runner token."
    log "Runner token generated."

    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${PG_CONTAINER}$"; then
      # Running on Unraid — deploy runner directly
      info "Setting up GitHub Actions runner..."
      mkdir -p "${RUNNER_DIR}"

      printf '%s\n' \
        'services:' \
        '  github-runner:' \
        '    image: myoung34/github-runner:latest' \
        "    container_name: github-runner-${PROJECT_NAME}" \
        '    restart: unless-stopped' \
        '    environment:' \
        "      RUNNER_NAME: ${PROJECT_NAME}-runner" \
        '      RUNNER_TOKEN: ${RUNNER_TOKEN}' \
        '      REPO_URL: ${REPO_URL}' \
        "      RUNNER_WORKDIR: /tmp/github-runner-${PROJECT_NAME}" \
        "      LABELS: ${PROJECT_NAME},self-hosted,linux" \
        '    volumes:' \
        '      - /var/run/docker.sock:/var/run/docker.sock' \
        "      - /tmp/github-runner-${PROJECT_NAME}:/tmp/github-runner-${PROJECT_NAME}" \
        '    networks:' \
        '      - dev_network' \
        '' \
        'networks:' \
        '  dev_network:' \
        '    external: true' \
        > "${RUNNER_DIR}/docker-compose.yml"

      printf '%s\n' \
        "RUNNER_TOKEN=${RUNNER_TOKEN}" \
        "REPO_URL=https://github.com/${GITHUB_OWNER}/${PROJECT_NAME}" \
        > "${RUNNER_DIR}/.env"

      docker compose -f "${RUNNER_DIR}/docker-compose.yml" \
        --env-file "${RUNNER_DIR}/.env" up -d \
        && log "Runner container started." \
        || error "Failed to start runner container."
    else
      # Running on dev machine — print instructions for Unraid
      warn "Not running on Unraid — cannot deploy runner container automatically."
      echo ""
      echo "  To deploy the runner on Unraid, create the following files and add"
      echo "  the stack to Compose Manager:"
      echo ""
      echo "  ${APPDATA_BASE}/github-runners/${PROJECT_NAME}/docker-compose.yml"
      echo "  ${APPDATA_BASE}/github-runners/${PROJECT_NAME}/.env"
      echo ""
      echo "  .env contents:"
      echo "    RUNNER_TOKEN=${RUNNER_TOKEN}"
      echo "    REPO_URL=https://github.com/${GITHUB_OWNER}/${PROJECT_NAME}"
      echo ""
      echo "  Note: This token expires in 1 hour."
      echo ""
    fi
  fi

  # --- Step 9: Initial commit ---
  if git status --porcelain | grep -q "^"; then
    info "Committing scaffolded files..."
    git add .
    git commit -m "chore: initial project scaffold"
    git push origin main
    log "Initial commit pushed."
  else
    warn "Nothing to commit — skipping initial commit."
  fi

  # --- Done ---
  echo ""
  echo "============================================"
  echo -e "  ${GREEN}Project '${PROJECT_NAME}' is ready!${NC}"
  echo "============================================"
  echo ""
  echo "  Repo:       ${REPO_HTTPS_URL}"
  echo "  Database:   ${DB_NAME} (user: ${DB_USER})"
  if [[ "$IS_SECONDARY" == "y" ]]; then
    echo "  Project:    Create manually on 10.0.2.166 via Dockhand"
    echo "  DB URL:     postgresql://${DB_USER}:***@10.0.0.X:5432/${DB_NAME}?sslmode=disable"
  else
    echo "  Project:    ${PROJECT_DIR}"
    echo "  DB URL:     postgresql://${DB_USER}:***@pg18:5432/${DB_NAME}?sslmode=disable"
  fi
  echo "  Runner:     ${RUNNER_DIR}"
  echo ""
  echo "  Next steps:"
  echo "  1. cd ${PROJECT_NAME} and start building"
  echo "  2. Verify runner is Idle in GitHub → Settings → Actions → Runners"
  echo "  3. Update ${PROJECT_DIR}/docker-compose.yml with your actual image tag after first deploy"
  echo "  4. Run ./project.sh from inside the repo when ready to deploy"
  echo ""
  exit 0
fi

# ============================================================
# DEPLOY MODE
# ============================================================

echo ""
echo "--- Deploy Mode ---"
echo ""

# --- Verify we're on main ---
CURRENT_BRANCH=$(git branch --show-current)
[[ "$CURRENT_BRANCH" != "main" ]] && error "You must be on the main branch to deploy. Currently on: ${CURRENT_BRANCH}"

# --- Verify working tree is clean ---
[[ -n $(git status --porcelain) ]] && error "Working tree is dirty. Commit or stash your changes before deploying."

# --- Verify up to date with remote ---
info "Fetching latest from remote..."
git fetch origin main --tags

# Ensure tracking branch is set
git branch --set-upstream-to=origin/main main &>/dev/null || true

LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/main)
[[ "$LOCAL" != "$REMOTE" ]] && error "Your branch is behind origin/main. Run: git pull origin main"

# --- Determine current version ---
LATEST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)

if [[ -z "$LATEST_TAG" ]]; then
  warn "No existing version tags found. Starting at v0.1.0."
  NEW_TAG="v0.1.0"
else
  log "Current version: ${LATEST_TAG}"
  VERSION="${LATEST_TAG#v}"
  MAJOR=$(echo "$VERSION" | cut -d. -f1)
  MINOR=$(echo "$VERSION" | cut -d. -f2)
  PATCH=$(echo "$VERSION" | cut -d. -f3)
  NEW_PATCH=$((PATCH + 1))
  NEW_TAG="v${MAJOR}.${MINOR}.${NEW_PATCH}"
fi

log "New version: ${NEW_TAG}"

# --- Confirm ---
echo ""
read -p "Deploy ${NEW_TAG} to ${REPO}? (y/n): " CONFIRM
[[ "$CONFIRM" != "y" ]] && echo "Aborted." && exit 0

echo ""

# --- Tag and push ---
info "Creating tag ${NEW_TAG}..."
git tag "${NEW_TAG}"

info "Pushing tag to origin..."
git push origin "${NEW_TAG}"
log "Tag pushed. Pipeline triggered."

# --- Wait for pipeline to start ---
echo ""
info "Waiting for GitHub Actions pipeline to start..."
sleep 10

ELAPSED=0
RUN_ID=""

while [[ -z "$RUN_ID" ]]; do
  RUN_ID=$(gh run list \
    --repo "${REPO}" \
    --event push \
    --limit 5 \
    --json databaseId,headBranch,status,createdAt \
    --jq "[.[] | select(.headBranch == \"${NEW_TAG}\")] | first | .databaseId // empty")

  if [[ -z "$RUN_ID" ]]; then
    ELAPSED=$((ELAPSED + 5))
    if [[ $ELAPSED -ge 60 ]]; then
      git tag -d "${NEW_TAG}"
      git push origin ":refs/tags/${NEW_TAG}"
      error "Timed out waiting for pipeline to start. Tag has been removed. Check GitHub Actions manually."
    fi
    sleep 5
  fi
done

log "Pipeline started. Run ID: ${RUN_ID}"
echo ""

# --- Poll pipeline status ---
ELAPSED=0
while true; do
  JOB_SUMMARY=$(gh run view "${RUN_ID}" \
    --repo "${REPO}" \
    --json jobs \
    --jq '.jobs[] | "  \(.name): \(.status)\(if .conclusion != null and .conclusion != "" then " (\(.conclusion))" else "" end)"')

  STATUS=$(gh run view "${RUN_ID}" \
    --repo "${REPO}" \
    --json status \
    --jq '.status')

  CONCLUSION=$(gh run view "${RUN_ID}" \
    --repo "${REPO}" \
    --json conclusion \
    --jq '.conclusion')

  echo -e "${BLUE}[→]${NC} Pipeline status: ${STATUS}"
  echo "$JOB_SUMMARY" | while read -r line; do
    if echo "$line" | grep -q "success"; then
      echo -e "  ${GREEN}${line}${NC}"
    elif echo "$line" | grep -q "failure\|cancelled"; then
      echo -e "  ${RED}${line}${NC}"
    else
      echo -e "  ${YELLOW}${line}${NC}"
    fi
  done

  if [[ "$STATUS" == "completed" ]]; then
    echo ""
    if [[ "$CONCLUSION" == "success" ]]; then
      log "Pipeline completed successfully."
      echo ""
      echo "============================================"
      echo -e "  ${GREEN}${NEW_TAG} is ready to deploy!${NC}"
      echo "============================================"
      echo ""
      echo "  Image: ghcr.io/${REPO}:${NEW_TAG#v}"
      echo ""
      echo "  To deploy on Unraid:"
      echo "  1. Update image tag in /mnt/user/appdata/your-project/docker-compose.yml"
      echo "  2. Run:"
      echo "     docker compose -f /mnt/user/appdata/your-project/docker-compose.yml pull"
      echo "     docker compose -f /mnt/user/appdata/your-project/docker-compose.yml up -d"
      echo ""
      echo "  To deploy on secondary server (10.0.2.166):"
      echo "  1. Update image tag in Dockhand and redeploy"
      echo ""
      echo "  View pipeline: $(gh run view ${RUN_ID} --repo ${REPO} --json url --jq '.url')"
      echo ""
    else
      git tag -d "${NEW_TAG}"
      git push origin ":refs/tags/${NEW_TAG}"
      error "Pipeline failed with conclusion: ${CONCLUSION}. Tag has been removed.
      Check GitHub Actions: $(gh run view ${RUN_ID} --repo ${REPO} --json url --jq '.url')"
    fi
    break
  fi

  ELAPSED=$((ELAPSED + POLL_INTERVAL))
  if [[ $ELAPSED -ge $POLL_TIMEOUT ]]; then
    echo ""
    warn "Timed out waiting for pipeline after ${POLL_TIMEOUT}s."
    warn "Check pipeline manually: $(gh run view ${RUN_ID} --repo ${REPO} --json url --jq '.url')"
    exit 1
  fi

  sleep $POLL_INTERVAL
  echo ""
done