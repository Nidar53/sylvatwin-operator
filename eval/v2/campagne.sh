#!/bin/bash
# Usage : campagne.sh [--reps N] [--scenario X]... [--dry-run] [--seed S]
cd "$(dirname "$0")" && exec python3 campagne.py "$@"
