<script lang="ts">
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';

  interface Props {
    body: string;
  }

  let { body }: Props = $props();

  // Configure marked for terminal-friendly rendering
  marked.setOptions({
    breaks: true,
    gfm: true,
  });

  let html = $derived(DOMPurify.sanitize(marked.parse(body) as string));
</script>

<div class="prose-terminal font-mono text-sm text-terminal-fg">
  {@html html}
</div>

<style>
  /* Terminal-styled markdown output */
  .prose-terminal :global(h1),
  .prose-terminal :global(h2),
  .prose-terminal :global(h3),
  .prose-terminal :global(h4),
  .prose-terminal :global(h5),
  .prose-terminal :global(h6) {
    color: var(--color-accent-500);
    font-weight: 600;
    margin-top: 1em;
    margin-bottom: 0.5em;
    font-size: inherit;
  }

  .prose-terminal :global(h1) {
    font-size: 1.1em;
  }

  .prose-terminal :global(h2) {
    font-size: 1.05em;
  }

  .prose-terminal :global(p) {
    margin-bottom: 0.75em;
    line-height: 1.6;
  }

  .prose-terminal :global(a) {
    color: var(--color-accent-500);
    text-decoration: underline;
  }

  .prose-terminal :global(a:hover) {
    color: var(--color-accent-400);
  }

  .prose-terminal :global(code) {
    background: var(--color-terminal-surface);
    border: 1px solid var(--color-terminal-border);
    padding: 0.1em 0.3em;
    border-radius: 2px;
    font-size: 0.9em;
  }

  .prose-terminal :global(pre) {
    background: var(--color-terminal-surface);
    border: 1px solid var(--color-terminal-border);
    padding: 0.75em;
    overflow-x: auto;
    margin-bottom: 0.75em;
    border-radius: 2px;
  }

  .prose-terminal :global(pre code) {
    background: none;
    border: none;
    padding: 0;
  }

  .prose-terminal :global(blockquote) {
    border-left: 2px solid var(--color-terminal-border);
    padding-left: 0.75em;
    color: var(--color-terminal-dim);
    margin-bottom: 0.75em;
  }

  .prose-terminal :global(ul),
  .prose-terminal :global(ol) {
    padding-left: 1.5em;
    margin-bottom: 0.75em;
  }

  .prose-terminal :global(li) {
    margin-bottom: 0.25em;
  }

  .prose-terminal :global(hr) {
    border-color: var(--color-terminal-border);
    margin: 1em 0;
  }

  .prose-terminal :global(img) {
    max-width: 100%;
    border: 1px solid var(--color-terminal-border);
    border-radius: 2px;
  }

  .prose-terminal :global(table) {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 0.75em;
    font-size: 0.9em;
  }

  .prose-terminal :global(th),
  .prose-terminal :global(td) {
    border: 1px solid var(--color-terminal-border);
    padding: 0.3em 0.6em;
    text-align: left;
  }

  .prose-terminal :global(th) {
    background: var(--color-terminal-surface);
    color: var(--color-accent-500);
  }
</style>
