#!/bin/bash

if [ -z "$GITHUB_ACTIONS" ]
then
  echo "not setting up git as not in a GitHub Action"
else
  echo "lets setup git"
  git config user.name github-actions
  git config user.email github-actions@github.com
fi


make regen

echo "ran the generator"

if [ -z "$DISABLE_COMMIT" ]
then
    echo "adding generated content"
    git commit -a -m "chore: regenerated plugin docs"
    git push
else
    echo "disabled commiting changes"
fi

echo "complete"
