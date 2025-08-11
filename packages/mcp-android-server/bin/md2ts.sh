#!/usr/bin/env bash

set -e -o pipefail

process_file() {
    local file=$1

    value=$(cat "$file")
    # Escape backslashes
    value=${value//\\/\\\\}
    # Escape backticks
    value=${value//\`/\\\`}
    # Handle interpolation operator ${
    value=${value//\$\{/\\\$\{}
    # Use perl to handle { and }, but not {% and %}
    value=$(echo "$value" | perl -pe 's/\{(?!%)/{{/g' | perl -pe 's/(?<!%)\}/}}/g')
    # Handle {% and %}
    value=${value//\{%/\{}
    value=${value//%\}/\}}

    filename=$(basename "$file" .md)
    # Convert to camelCase naming
    name=$(echo "$filename" | perl -pe 's/(-)(\w)/\U$2/g')

    target_file="${file%.md}.ts"
    source_file="./$(basename "$file")"

    echo "// Generated from '$source_file'" > "$target_file"
    echo -e "// Edit '$source_file' and run \`npm run md2ts\`, if you want to modify this file.\n" >> "$target_file"
    printf "export const %s = \`%s\n\`;\n" "${name}Tpl" "$value" >> "$target_file"
}

process_directory() {
    local dir=$1

    files=$(find "$dir" -type f -name "*.md")

    for file in $files; do
        process_file "$file"
    done
}

# If no arguments are provided, search for prompt-*.md files in src directory
if [ $# -eq 0 ]; then
    echo "No arguments provided, searching for prompt-*.md files in src directory and all .md files in src/agents/*/prompts"
    # Find prompt-*.md files in src directory
    prompt_files=$(find src -type f -name "prompt-*.md")

    # Find all .md files in src/agents/*/prompts
    agents_prompt_files=$(find src -path '*/prompts/*.md' -type f -not -path '*/tests/__fixtures__/*' -not -path 'src/core/*' 2>/dev/null || true)

    # Process all found files
    for file in $prompt_files $agents_prompt_files; do
        process_file "$file"
    done
else
    # Process arguments as files or directories
    for arg in "$@"; do
        if [ -f "$arg" ]; then
            process_file "$arg"
        elif [ -d "$arg" ]; then
            process_directory "$arg"
        else
            echo "Warning: $arg is neither a file nor a directory, skipping"
        fi
    done
fi
