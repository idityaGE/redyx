<script lang="ts">
  import { onMount } from 'svelte';

  interface Props {
    images: string[];
    startIndex: number;
    onClose: () => void;
  }

  let { images, startIndex, onClose }: Props = $props();

  let currentIndex = $state(startIndex);

  function prev() {
    currentIndex = currentIndex <= 0 ? images.length - 1 : currentIndex - 1;
  }

  function next() {
    currentIndex = currentIndex >= images.length - 1 ? 0 : currentIndex + 1;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
    else if (e.key === 'ArrowLeft') prev();
    else if (e.key === 'ArrowRight') next();
  }

  function handleBackdropClick(e: MouseEvent) {
    // Close if clicking the backdrop (not the image or controls)
    if (e.target === e.currentTarget) {
      onClose();
    }
  }

  onMount(() => {
    document.addEventListener('keydown', handleKeydown);
    // Prevent body scrolling
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', handleKeydown);
      document.body.style.overflow = '';
    };
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 bg-black/90 flex items-center justify-center font-mono"
  onclick={handleBackdropClick}
>
  <!-- Close button -->
  <button
    class="absolute top-4 right-4 text-terminal-dim hover:text-terminal-fg text-sm cursor-pointer z-10 transition-colors"
    onclick={onClose}
  >
    [x] close
  </button>

  <!-- Navigation: prev -->
  {#if images.length > 1}
    <button
      class="absolute left-4 top-1/2 -translate-y-1/2 text-terminal-dim hover:text-terminal-fg
        text-xl cursor-pointer z-10 transition-colors px-2 py-4"
      onclick={prev}
    >
      &lt;
    </button>
  {/if}

  <!-- Image -->
  <img
    src={images[currentIndex]}
    alt="Image {currentIndex + 1} of {images.length}"
    class="max-w-[90vw] max-h-[90vh] object-contain select-none"
  />

  <!-- Navigation: next -->
  {#if images.length > 1}
    <button
      class="absolute right-4 top-1/2 -translate-y-1/2 text-terminal-dim hover:text-terminal-fg
        text-xl cursor-pointer z-10 transition-colors px-2 py-4"
      onclick={next}
    >
      &gt;
    </button>
  {/if}

  <!-- Counter -->
  {#if images.length > 1}
    <div class="absolute bottom-4 left-1/2 -translate-x-1/2 text-terminal-dim text-xs">
      {currentIndex + 1} / {images.length}
    </div>
  {/if}
</div>
