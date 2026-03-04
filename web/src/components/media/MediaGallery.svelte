<script lang="ts">
  import Lightbox from './Lightbox.svelte';

  interface Props {
    mediaUrls: string[];
    thumbnailUrl?: string;
  }

  let { mediaUrls, thumbnailUrl }: Props = $props();

  let lightboxOpen = $state(false);
  let lightboxIndex = $state(0);

  function isVideoUrl(url: string): boolean {
    const lower = url.toLowerCase();
    return lower.endsWith('.mp4') || lower.endsWith('.webm') || lower.includes('video');
  }

  let imageUrls = $derived(mediaUrls.filter(u => !isVideoUrl(u)));
  let videoUrls = $derived(mediaUrls.filter(u => isVideoUrl(u)));

  function openLightbox(index: number) {
    lightboxIndex = index;
    lightboxOpen = true;
  }
</script>

{#if mediaUrls.length > 0}
  <div class="space-y-2">
    <!-- Images: stacked vertically -->
    {#each imageUrls as url, i}
      <button
        class="block w-full cursor-pointer border border-terminal-border hover:border-accent-500 transition-colors bg-transparent p-0"
        onclick={() => openLightbox(i)}
      >
        <img
          src={url}
          alt="Post image {i + 1}"
          class="max-w-full"
          loading="lazy"
        />
      </button>
    {/each}

    <!-- Videos: native player -->
    {#each videoUrls as url}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video
        src={url}
        controls
        class="max-w-full border border-terminal-border"
        preload="metadata"
      >
        <track kind="captions" />
      </video>
    {/each}
  </div>

  <!-- Lightbox overlay -->
  {#if lightboxOpen && imageUrls.length > 0}
    <Lightbox
      images={imageUrls}
      startIndex={lightboxIndex}
      onClose={() => { lightboxOpen = false; }}
    />
  {/if}
{/if}
