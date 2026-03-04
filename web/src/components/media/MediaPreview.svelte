<script lang="ts">
  interface PreviewFile {
    id: string;
    file: File;
    status: 'uploading' | 'complete' | 'error';
    thumbnailUrl: string | null;
    url: string | null;
  }

  interface Props {
    files: PreviewFile[];
    onRemove: (index: number) => void;
  }

  let { files, onRemove }: Props = $props();

  function isVideo(type: string): boolean {
    return type.startsWith('video/');
  }
</script>

{#if files.length > 0}
  <div class="mt-2">
    <div class="text-xs text-terminal-dim font-mono mb-1">&gt; previews:</div>
    <div class="flex flex-wrap gap-2">
      {#each files as file, i}
        <div class="relative border border-terminal-border bg-terminal-bg group">
          {#if isVideo(file.file.type)}
            <!-- Video placeholder -->
            <div class="w-20 h-20 flex items-center justify-center text-terminal-dim font-mono text-xs">
              <div class="text-center">
                <div class="text-lg">&#9654;</div>
                <div>video</div>
              </div>
            </div>
          {:else if file.thumbnailUrl}
            <img
              src={file.thumbnailUrl}
              alt={file.file.name}
              class="w-20 h-20 object-cover"
            />
          {:else if file.url}
            <img
              src={file.url}
              alt={file.file.name}
              class="w-20 h-20 object-cover"
            />
          {:else}
            <div class="w-20 h-20 flex items-center justify-center text-terminal-dim font-mono text-xs">
              img
            </div>
          {/if}

          <!-- Remove button overlay -->
          <button
            class="absolute -top-1.5 -right-1.5 w-5 h-5 bg-terminal-surface border border-terminal-border
              text-terminal-dim hover:text-red-500 hover:border-red-500 text-xs font-mono
              flex items-center justify-center cursor-pointer transition-colors"
            onclick={() => onRemove(i)}
            title="Remove"
          >
            x
          </button>
        </div>
      {/each}
    </div>
  </div>
{/if}
