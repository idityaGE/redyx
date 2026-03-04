<script lang="ts">
  import { api } from '../../lib/api';
  import { getAccessToken } from '../../lib/api';
  import MediaPreview from './MediaPreview.svelte';

  interface Props {
    onMediaChange: (mediaIds: string[]) => void;
  }

  let { onMediaChange }: Props = $props();

  type UploadFile = {
    id: string;
    file: File;
    progress: number;
    status: 'uploading' | 'complete' | 'error';
    mediaId: string | null;
    url: string | null;
    thumbnailUrl: string | null;
    error: string | null;
    xhr: XMLHttpRequest | null;
  };

  const MAX_IMAGE_SIZE = 20 * 1024 * 1024; // 20MB
  const MAX_VIDEO_SIZE = 100 * 1024 * 1024; // 100MB
  const MAX_IMAGES = 5;
  const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
  const ALLOWED_VIDEO_TYPES = ['video/mp4', 'video/webm'];
  const ACCEPT_STRING = [...ALLOWED_IMAGE_TYPES, ...ALLOWED_VIDEO_TYPES].join(',');

  let files = $state<UploadFile[]>([]);
  let dragOver = $state(false);
  let fileInput: HTMLInputElement;

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  function isVideo(type: string): boolean {
    return ALLOWED_VIDEO_TYPES.includes(type);
  }

  function isImage(type: string): boolean {
    return ALLOWED_IMAGE_TYPES.includes(type);
  }

  function buildProgressBar(progress: number): string {
    const barWidth = 30;
    const filled = Math.round((progress / 100) * barWidth);
    const empty = barWidth - filled;
    const bar = '='.repeat(Math.max(0, filled - 1)) + (filled > 0 ? '>' : '') + ' '.repeat(empty);
    return `[${bar}] ${progress}%`;
  }

  function generateId(): string {
    return Math.random().toString(36).substring(2, 10);
  }

  function validateFile(file: File, existingFiles: UploadFile[]): string | null {
    const allTypes = [...existingFiles.filter(f => f.status !== 'error'), { file }];

    // Check format
    if (!isImage(file.type) && !isVideo(file.type)) {
      return `> error: unsupported format (${file.type || 'unknown'})`;
    }

    // Check size
    if (isImage(file.type) && file.size > MAX_IMAGE_SIZE) {
      return `> error: file too large (${formatSize(file.size)} / ${formatSize(MAX_IMAGE_SIZE)} max)`;
    }
    if (isVideo(file.type) && file.size > MAX_VIDEO_SIZE) {
      return `> error: file too large (${formatSize(file.size)} / ${formatSize(MAX_VIDEO_SIZE)} max)`;
    }

    // Check mixing rule
    const hasImages = existingFiles.some(f => f.status !== 'error' && isImage(f.file.type));
    const hasVideos = existingFiles.some(f => f.status !== 'error' && isVideo(f.file.type));

    if (isVideo(file.type) && hasImages) {
      return '> error: cannot mix video and images — remove images first';
    }
    if (isImage(file.type) && hasVideos) {
      return '> error: cannot mix images and video — remove video first';
    }
    if (isVideo(file.type) && hasVideos) {
      return '> error: only one video allowed';
    }

    // Check image limit
    const imageCount = existingFiles.filter(f => f.status !== 'error' && isImage(f.file.type)).length;
    if (isImage(file.type) && imageCount >= MAX_IMAGES) {
      return `> error: maximum ${MAX_IMAGES} images allowed`;
    }

    return null;
  }

  function notifyParent() {
    const completedIds = files
      .filter(f => f.status === 'complete' && f.mediaId)
      .map(f => f.mediaId!);
    onMediaChange(completedIds);
  }

  async function uploadFile(entry: UploadFile) {
    try {
      // Step 1: Init upload to get presigned URL
      const mediaType = isVideo(entry.file.type) ? 'MEDIA_TYPE_VIDEO' : 'MEDIA_TYPE_IMAGE';
      const initRes = await api<{ mediaId: string; uploadUrl: string; expiresAt: string }>(
        '/media/upload',
        {
          method: 'POST',
          body: JSON.stringify({
            filename: entry.file.name,
            contentType: entry.file.type,
            sizeBytes: String(entry.file.size),
            mediaType,
          }),
        }
      );

      entry.mediaId = initRes.mediaId;

      // Step 2: PUT file to presigned URL using XMLHttpRequest for progress
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        entry.xhr = xhr;

        xhr.open('PUT', initRes.uploadUrl);
        xhr.setRequestHeader('Content-Type', entry.file.type);

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const idx = files.findIndex(f => f.id === entry.id);
            if (idx !== -1) {
              files[idx].progress = Math.round((e.loaded / e.total) * 100);
            }
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve();
          } else {
            reject(new Error(`Upload failed with status ${xhr.status}`));
          }
        };

        xhr.onerror = () => reject(new Error('Upload failed'));
        xhr.onabort = () => reject(new Error('Upload cancelled'));

        xhr.send(entry.file);
      });

      // Step 3: Complete upload
      const completeRes = await api<{ url: string; thumbnailUrl: string; status: string }>(
        `/media/${initRes.mediaId}/complete`,
        { method: 'POST' }
      );

      const idx = files.findIndex(f => f.id === entry.id);
      if (idx !== -1) {
        files[idx].status = 'complete';
        files[idx].url = completeRes.url;
        files[idx].thumbnailUrl = completeRes.thumbnailUrl;
        files[idx].progress = 100;
        files[idx].xhr = null;
      }

      notifyParent();
    } catch (e) {
      const idx = files.findIndex(f => f.id === entry.id);
      if (idx !== -1) {
        files[idx].error = `> error: ${e instanceof Error ? e.message : 'upload failed'}`;
        files[idx].status = 'error';
        files[idx].xhr = null;
      }
    }
  }

  function processFiles(newFiles: FileList | File[]) {
    const fileArray = Array.from(newFiles);

    for (const file of fileArray) {
      const validationError = validateFile(file, files);

      const entry: UploadFile = {
        id: generateId(),
        file,
        progress: 0,
        status: validationError ? 'error' : 'uploading',
        mediaId: null,
        url: null,
        thumbnailUrl: null,
        error: validationError,
        xhr: null,
      };

      files = [...files, entry];

      if (!validationError) {
        uploadFile(entry);
      }
    }
  }

  function handleRemove(index: number) {
    const entry = files[index];
    // Cancel in-progress upload
    if (entry.xhr) {
      entry.xhr.abort();
    }
    files = files.filter((_, i) => i !== index);
    notifyParent();
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    if (e.dataTransfer?.files) {
      processFiles(e.dataTransfer.files);
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    dragOver = true;
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
  }

  function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files) {
      processFiles(input.files);
      input.value = '';
    }
  }

  function openFilePicker() {
    fileInput?.click();
  }
</script>

<div class="space-y-3">
  <!-- Drop zone -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="border-2 border-dashed rounded p-6 text-center cursor-pointer transition-colors
      {dragOver
        ? 'border-accent-500 bg-terminal-bg'
        : 'border-terminal-border hover:border-terminal-dim'}"
    ondrop={handleDrop}
    ondragover={handleDragOver}
    ondragleave={handleDragLeave}
    onclick={openFilePicker}
    role="button"
    tabindex="0"
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') openFilePicker(); }}
  >
    <div class="text-sm text-terminal-dim font-mono">
      [ drop files here or click to browse ]
    </div>
    <div class="text-xs text-terminal-dim mt-2 font-mono">
      images: jpg, png, gif, webp (max 20MB) &middot; video: mp4, webm (max 100MB)
    </div>
    <div class="text-xs text-terminal-dim font-mono">
      up to {MAX_IMAGES} images OR 1 video
    </div>
  </div>

  <input
    bind:this={fileInput}
    type="file"
    multiple
    accept={ACCEPT_STRING}
    onchange={handleFileSelect}
    class="hidden"
  />

  <!-- Upload progress list -->
  {#if files.length > 0}
    <div class="space-y-2">
      {#each files as entry, i}
        <div class="font-mono text-xs border border-terminal-border p-2 bg-terminal-bg">
          <div class="flex items-center justify-between">
            <span class="text-terminal-fg truncate">
              {entry.file.name}
              <span class="text-terminal-dim">({formatSize(entry.file.size)})</span>
            </span>
            <button
              class="text-terminal-dim hover:text-red-500 transition-colors cursor-pointer ml-2 flex-shrink-0"
              onclick={() => handleRemove(i)}
              title="Remove"
            >
              [x]
            </button>
          </div>

          {#if entry.status === 'uploading'}
            <div class="text-accent-500 mt-1">
              {buildProgressBar(entry.progress)}
            </div>
          {:else if entry.status === 'error'}
            <div class="text-red-500 mt-1">
              {entry.error}
            </div>
          {:else if entry.status === 'complete'}
            <div class="text-green-500 mt-1">
              &gt; upload complete
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Thumbnail previews for completed uploads -->
    {#if files.some(f => f.status === 'complete')}
      <MediaPreview
        files={files.filter(f => f.status === 'complete')}
        onRemove={(index) => {
          const completeFiles = files.filter(f => f.status === 'complete');
          const targetFile = completeFiles[index];
          const realIndex = files.findIndex(f => f.id === targetFile.id);
          if (realIndex !== -1) handleRemove(realIndex);
        }}
      />
    {/if}
  {/if}
</div>
