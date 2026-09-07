#!/usr/bin/env bash

dirs=$(fd "run-.*" --no-ignore --hidden --type directory)
session=$(gum filter $dirs)
for f in "$session"/gh-enhance-frame-*; do
  cat "$f"
  sleep 0.05
done

echo "done"
