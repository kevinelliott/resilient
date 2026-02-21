#!/bin/bash
rm -rf .git
git init
git add .
git commit -m "Initial commit for Resilient Mesh Daemon phase 8"
git branch -M main
gh repo create kevinelliott/resilient --public --source=. --remote=origin --push || {
    git remote add origin https://github.com/kevinelliott/resilient.git || true
    git push -u origin main -f
}
