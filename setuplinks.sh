#!/bin/bash

# linking .gitconfig.local
if [ ! -e $HOME/.gitconfig.local ]; then
    if [ -f .gitconfig.local.github ];  then
        echo "Linking .gitconfig.local.github"
        ln -s $PWD/.gitconfig.local.github $HOME/.gitconfig.local
    fi
fi

# linking .npmrc
if [ ! -e $HOME/.npmrc ]; then
    if [ -f .npmrc ];  then
        echo "Linking .npmrc"
        ln -s $PWD/.npmrc $HOME/.npmrc
    fi
fi

# linking .cobra.yaml
if [ ! -e $HOME/.cobra.yaml ]; then
    echo "Linking .cobra.yaml"
    ln -s $PWD/.cobra.yaml $HOME/.cobra.yaml
fi
