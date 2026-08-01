<template>
  <div class="structured-editor" :class="{ 'structured-editor--disabled': disabled }">
    <div class="structured-editor__heading">
      <div>
        <span class="structured-editor__label">{{ label }}</span>
        <small>Structured content · headings are limited to H2–H4</small>
      </div>
      <span class="structured-editor__mode">Visual editor</span>
    </div>

    <div v-if="editor" class="structured-editor__toolbar" role="toolbar" aria-label="Article formatting">
      <select
        class="structured-editor__select"
        aria-label="Text style"
        :disabled="disabled"
        :value="activeTextStyle"
        @change="setTextStyle(($event.target as HTMLSelectElement).value)"
      >
        <option value="paragraph">Paragraph</option>
        <option value="heading-2">Heading 2</option>
        <option value="heading-3">Heading 3</option>
        <option value="heading-4">Heading 4</option>
      </select>

      <span class="structured-editor__group" aria-label="Text formatting">
        <button type="button" :class="buttonClass('bold')" :aria-pressed="editor.isActive('bold')" :disabled="disabled" aria-label="Bold" title="Bold (⌘B)" @click="run('toggleBold')"><Bold :size="17" /></button>
        <button type="button" :class="buttonClass('italic')" :aria-pressed="editor.isActive('italic')" :disabled="disabled" aria-label="Italic" title="Italic (⌘I)" @click="run('toggleItalic')"><Italic :size="17" /></button>
        <button type="button" :class="buttonClass('underline')" :aria-pressed="editor.isActive('underline')" :disabled="disabled" aria-label="Underline" title="Underline (⌘U)" @click="run('toggleUnderline')"><Underline :size="17" /></button>
        <button type="button" :class="buttonClass('strike')" :aria-pressed="editor.isActive('strike')" :disabled="disabled" aria-label="Strikethrough" title="Strikethrough" @click="run('toggleStrike')"><Strikethrough :size="17" /></button>
        <button type="button" :class="buttonClass('code')" :aria-pressed="editor.isActive('code')" :disabled="disabled" aria-label="Inline code" title="Inline code" @click="run('toggleCode')"><Code2 :size="17" /></button>
      </span>

      <span class="structured-editor__group" aria-label="Links">
        <button type="button" :class="buttonClass('link')" :aria-pressed="editor.isActive('link')" :disabled="disabled" aria-label="Add or edit link" title="Add or edit link" @click="editLink"><Link2 :size="17" /></button>
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.isActive('link')" aria-label="Remove link" title="Remove link" @click="editor.chain().focus().unsetLink().run()"><Unlink2 :size="17" /></button>
      </span>

      <span class="structured-editor__group" aria-label="Blocks">
        <button type="button" :class="buttonClass('bulletList')" :aria-pressed="editor.isActive('bulletList')" :disabled="disabled" aria-label="Bulleted list" title="Bulleted list" @click="run('toggleBulletList')"><List :size="17" /></button>
        <button type="button" :class="buttonClass('orderedList')" :aria-pressed="editor.isActive('orderedList')" :disabled="disabled" aria-label="Numbered list" title="Numbered list" @click="run('toggleOrderedList')"><ListOrdered :size="17" /></button>
        <button type="button" :class="buttonClass('blockquote')" :aria-pressed="editor.isActive('blockquote')" :disabled="disabled" aria-label="Block quote" title="Block quote" @click="run('toggleBlockquote')"><Quote :size="17" /></button>
        <button type="button" :class="buttonClass('codeBlock')" :aria-pressed="editor.isActive('codeBlock')" :disabled="disabled" aria-label="Code block" title="Code block" @click="run('toggleCodeBlock')"><SquareCode :size="17" /></button>
      </span>

      <details ref="insertMenu" class="structured-editor__menu" :class="{ 'is-disabled': disabled }">
        <summary class="structured-editor__menu-trigger"><Plus :size="16" /> Insert <ChevronDown :size="14" /></summary>
        <div class="structured-editor__menu-panel">
          <p class="structured-editor__menu-label">Content</p>
          <button type="button" @click="insertFromMenu(insertImage)"><ImageIcon :size="17" /><span><strong>Image</strong><small>From a URL</small></span></button>
          <button type="button" @click="insertFromMenu(() => editor?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run())"><Table2 :size="17" /><span><strong>Table</strong><small>3 × 3 with header</small></span></button>
          <button type="button" @click="insertFromMenu(() => editor?.chain().focus().setHorizontalRule().run())"><Minus :size="17" /><span><strong>Divider</strong><small>Separate sections</small></span></button>
          <p class="structured-editor__menu-label">Article blocks</p>
          <button type="button" @click="insertEditorialFromMenu('callout')"><MessageSquareText :size="17" /><span><strong>Callout</strong><small>Highlight useful context</small></span></button>
          <button type="button" @click="insertEditorialFromMenu('takeaway')"><Lightbulb :size="17" /><span><strong>Key takeaway</strong><small>Summarize the main point</small></span></button>
          <button type="button" @click="insertEditorialFromMenu('steps')"><ListChecks :size="17" /><span><strong>Steps</strong><small>A guided process</small></span></button>
          <button type="button" @click="insertEditorialFromMenu('pros-cons')"><Scale :size="17" /><span><strong>Pros and cons</strong><small>Show tradeoffs</small></span></button>
          <button type="button" @click="insertEditorialFromMenu('cta')"><MousePointerClick :size="17" /><span><strong>Call to action</strong><small>Prompt a next step</small></span></button>
          <button type="button" @click="insertEditorialFromMenu('faq')"><CircleHelp :size="17" /><span><strong>FAQ</strong><small>Question and answer</small></span></button>
        </div>
      </details>

      <details ref="moreMenu" class="structured-editor__menu" :class="{ 'is-disabled': disabled }">
        <summary class="structured-editor__menu-trigger">More <ChevronDown :size="14" /></summary>
        <div class="structured-editor__menu-panel structured-editor__menu-panel--right">
          <p class="structured-editor__menu-label">Special blocks</p>
          <button type="button" @click="insertFromMenu(insertTaskList, moreMenu)"><ListTodo :size="17" /><span><strong>Task list</strong><small>Checklist items</small></span></button>
          <button type="button" @click="insertFromMenu(insertAttributedQuote, moreMenu)"><Quote :size="17" /><span><strong>Quote with citation</strong><small>Attributed quotation</small></span></button>
          <button type="button" @click="insertFromMenu(insertComparisonTable, moreMenu)"><Columns3 :size="17" /><span><strong>Comparison</strong><small>Compare options in a table</small></span></button>
          <button type="button" @click="insertFromMenu(insertGallery, moreMenu)"><GalleryHorizontalEnd :size="17" /><span><strong>Gallery</strong><small>Multiple project images</small></span></button>
          <button type="button" @click="insertFromMenu(insertTranscript, moreMenu)"><Captions :size="17" /><span><strong>Transcript</strong><small>Timestamped dialogue</small></span></button>
          <button type="button" @click="insertFromMenu(insertRelatedReference, moreMenu)"><BookOpen :size="17" /><span><strong>Related article</strong><small>Internal reading reference</small></span></button>
          <button type="button" @click="insertFromMenu(insertEmbed, moreMenu)"><PlaySquare :size="17" /><span><strong>Embed</strong><small>YouTube, Vimeo, or Wistia</small></span></button>
        </div>
      </details>

      <span class="structured-editor__spacer" />
      <span class="structured-editor__group structured-editor__group--history" aria-label="History">
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.can().chain().focus().undo().run()" aria-label="Undo" title="Undo (⌘Z)" @click="editor.chain().focus().undo().run()"><Undo2 :size="17" /></button>
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.can().chain().focus().redo().run()" aria-label="Redo" title="Redo (⇧⌘Z)" @click="editor.chain().focus().redo().run()"><Redo2 :size="17" /></button>
      </span>
    </div>

    <div v-if="editor && (mediaAssets.length || sources.length || editor.isActive('heading') || editor.isActive('table'))" class="structured-editor__context-bar">
      <span v-if="mediaAssets.length" class="structured-editor__picker">
        <ImageIcon :size="16" />
        <select v-model="selectedMediaID" class="structured-editor__select" aria-label="Project image" :disabled="disabled">
          <option value="">Choose project image…</option>
          <option v-for="asset in insertableMedia" :key="asset.id" :value="asset.id">{{ mediaLabel(asset) }}</option>
        </select>
        <button type="button" class="structured-editor__text-button" :disabled="disabled || !selectedMediaID" @click="insertSelectedMedia">Insert</button>
      </span>
      <span v-if="sources.length" class="structured-editor__picker">
        <BookOpen :size="16" />
        <select v-model="selectedSourceID" class="structured-editor__select" aria-label="Project source" :disabled="disabled">
          <option value="">Choose citation…</option>
          <option v-for="source in sources" :key="source.id" :value="source.id">{{ source.title }}</option>
        </select>
        <button type="button" class="structured-editor__text-button" :disabled="disabled || !selectedSourceID" @click="insertSelectedCitation">Cite</button>
      </span>
      <span v-if="editor.isActive('heading')" class="structured-editor__picker structured-editor__picker--context">
        <Anchor :size="16" />
        <button type="button" class="structured-editor__text-button" :disabled="disabled" @click="editHeadingAnchor">Edit heading anchor</button>
      </span>
      <span v-if="editor.isActive('table')" class="structured-editor__picker structured-editor__picker--context">
        <Table2 :size="16" />
        <span class="structured-editor__context-label">Table</span>
        <button type="button" class="structured-editor__text-button" :disabled="disabled" @click="editor.chain().focus().addRowAfter().run()">Add row</button>
        <button type="button" class="structured-editor__text-button" :disabled="disabled" @click="editor.chain().focus().addColumnAfter().run()">Add column</button>
        <button type="button" class="structured-editor__text-button" :disabled="disabled" @click="editor.chain().focus().deleteRow().run()">Remove row</button>
        <button type="button" class="structured-editor__text-button" :disabled="disabled" @click="editor.chain().focus().deleteColumn().run()">Remove column</button>
        <button type="button" class="structured-editor__text-button structured-editor__text-button--danger" :disabled="disabled" @click="editor.chain().focus().deleteTable().run()"><Trash2 :size="14" /> Delete</button>
      </span>
    </div>

    <form v-if="editorDialog" class="structured-editor__dialog" role="dialog" :aria-label="dialogTitle" @submit.prevent="applyEditorDialog">
      <div class="structured-editor__dialog-heading">
        <div>
          <strong>{{ dialogTitle }}</strong>
          <small>{{ dialogDescription }}</small>
        </div>
        <button type="button" aria-label="Close" title="Close" @click="closeEditorDialog"><X :size="17" /></button>
      </div>

      <label v-if="editorDialog === 'embed'">
        <span>Provider</span>
        <select v-model="dialogFields.provider">
          <option value="youtube">YouTube</option>
          <option value="vimeo">Vimeo</option>
          <option value="wistia">Wistia</option>
        </select>
      </label>
      <label v-if="editorDialog === 'link' || editorDialog === 'image' || editorDialog === 'embed'">
        <span>{{ editorDialog === 'image' ? 'Image URL' : 'URL' }}</span>
        <input v-model.trim="dialogFields.url" type="text" autofocus :placeholder="editorDialog === 'link' ? 'https://example.com or /page' : 'https://…'" />
      </label>
      <label v-if="editorDialog === 'image'">
        <span>Alt text <small>Leave blank only if decorative</small></span>
        <input v-model="dialogFields.alt" type="text" placeholder="Describe what the image shows" />
      </label>
      <label v-if="editorDialog === 'image'">
        <span>Caption <small>Optional</small></span>
        <input v-model="dialogFields.caption" type="text" placeholder="Add context below the image" />
      </label>
      <label v-if="editorDialog === 'anchor'">
        <span>Heading anchor</span>
        <div class="structured-editor__input-prefix"><span>#</span><input v-model.trim="dialogFields.anchor" type="text" autofocus placeholder="section-name" /></div>
      </label>
      <label v-if="editorDialog === 'related'">
        <span>Related article ID</span>
        <input v-model.trim="dialogFields.articleId" type="text" autofocus placeholder="article-id" />
      </label>

      <p v-if="dialogError" class="structured-editor__dialog-error" role="alert">{{ dialogError }}</p>
      <div class="structured-editor__dialog-actions">
        <button type="button" @click="closeEditorDialog">Cancel</button>
        <button v-if="editorDialog === 'link' && editor?.isActive('link')" type="button" class="structured-editor__dialog-remove" @click="removeLink">Remove link</button>
        <button type="submit" class="structured-editor__dialog-primary">{{ dialogActionLabel }}</button>
      </div>
    </form>

    <div class="structured-editor__canvas">
      <EditorContent v-if="editor" :editor="editor" />
      <div v-else class="structured-editor__loading" role="status">Loading editor…</div>
    </div>

    <p v-if="editorError" class="structured-editor__error" role="alert">{{ editorError }}</p>
    <p class="structured-editor__help">Images require an HTTPS or root-relative URL. Leave image alt text blank only when the image is decorative.</p>
  </div>
</template>

<script setup lang="ts">
import { Editor, EditorContent, Mark, Node, mergeAttributes } from '@tiptap/vue-3'
import Heading from '@tiptap/extension-heading'
import Image from '@tiptap/extension-image'
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table'
import StarterKit from '@tiptap/starter-kit'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import {
  Anchor,
  Bold,
  BookOpen,
  Captions,
  ChevronDown,
  CircleHelp,
  Code2,
  Columns3,
  GalleryHorizontalEnd,
  Image as ImageIcon,
  Italic,
  Lightbulb,
  Link2,
  List,
  ListChecks,
  ListOrdered,
  ListTodo,
  MessageSquareText,
  Minus,
  MousePointerClick,
  PlaySquare,
  Plus,
  Quote,
  Redo2,
  Scale,
  SquareCode,
  Strikethrough,
  Table2,
  Trash2,
  Underline,
  Undo2,
  Unlink2,
  X
} from 'lucide-vue-next'
import type { AdminMediaAsset, AdminSource } from '~/composables/useAdminApi'

const props = withDefaults(defineProps<{
  html: string
  bodyDocument?: unknown
  disabled?: boolean
  label?: string
  mediaAssets?: AdminMediaAsset[]
  sources?: AdminSource[]
}>(), {
  bodyDocument: undefined,
  disabled: false,
  label: 'Article body',
  mediaAssets: () => [],
  sources: () => []
})

const emit = defineEmits<{
  'update:html': [value: string]
  'update:bodyDocument': [value: unknown]
}>()

const editor = shallowRef<Editor | null>(null)
const editorError = ref('')
const selectedMediaID = ref('')
const selectedSourceID = ref('')
const insertMenu = ref<HTMLDetailsElement | null>(null)
const moreMenu = ref<HTMLDetailsElement | null>(null)
type EditorDialog = 'link' | 'image' | 'anchor' | 'related' | 'embed'
const editorDialog = ref<EditorDialog | null>(null)
const dialogError = ref('')
const dialogFields = reactive({ url: '', alt: '', caption: '', anchor: '', articleId: '', provider: 'youtube' })
const normalizeHeadingIDsKey = new PluginKey('normalize-article-heading-ids')
const insertableMedia = computed(() => props.mediaAssets.filter(asset => asset.status === 'ready' && asset.contentType.startsWith('image/') && isSafeEditorialURL(asset.url || '', false)))

const StableHeading = Heading.extend({
  addAttributes() {
    return {
      ...(this.parent?.() || {}),
      id: {
        default: null,
        parseHTML: element => element.getAttribute('id'),
        renderHTML: attributes => attributes.id ? { id: attributes.id } : {}
      }
    }
  },

  addProseMirrorPlugins() {
    const headingType = this.name
    return [
      new Plugin({
        key: new PluginKey('stable-article-heading-ids'),
        appendTransaction(transactions, _oldState, newState) {
          if (!transactions.some(transaction => transaction.docChanged || transaction.getMeta(normalizeHeadingIDsKey))) return null
          const used = new Set<string>()
          const updates: Array<{ position: number, id: string }> = []
          newState.doc.descendants((node, position) => {
            if (node.type.name !== headingType) return
            const requested = typeof node.attrs.id === 'string' ? node.attrs.id.trim() : ''
            const base = safeHeadingID(requested)
              ? requested
              : headingIDFromText(node.textContent)
            const id = uniqueHeadingID(base, used)
            if (requested !== id) updates.push({ position, id })
          })
          if (!updates.length) return null
          const transaction = newState.tr
          for (const update of updates) {
            const node = transaction.doc.nodeAt(update.position)
            if (node) transaction.setNodeMarkup(update.position, undefined, { ...node.attrs, id: update.id })
          }
          return transaction
        }
      })
    ]
  }
}).configure({ levels: [2, 3, 4] })

const AccessibleImage = Image.extend({
  parseHTML() {
    return [{
      tag: 'img[src]',
      getAttrs: element => isSafeEditorialURL((element as HTMLElement).getAttribute('src') || '', false) ? null : false
    }]
  },

  addAttributes() {
    return {
      ...(this.parent?.() || {}),
      decorative: {
        default: false,
        parseHTML: element => element.getAttribute('data-decorative') === 'true',
        renderHTML: attributes => attributes.decorative ? { 'data-decorative': 'true' } : {}
      }
    }
  }
}).configure({ allowBase64: false })

const Figure = Node.create({
  name: 'figure',
  group: 'block',
  content: 'image figcaption?',
  defining: true,
  parseHTML: () => [{ tag: 'figure' }],
  renderHTML: ({ HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes), 0]
})

const Figcaption = Node.create({
  name: 'figcaption',
  content: 'inline*',
  parseHTML: () => [{ tag: 'figcaption' }],
  renderHTML: ({ HTMLAttributes }) => ['figcaption', mergeAttributes(HTMLAttributes), 0]
})

const editorialBlockKinds = ['callout', 'takeaway', 'steps', 'pros-cons', 'cta', 'faq'] as const
type EditorialBlockKind = typeof editorialBlockKinds[number]

const EditorialBlock = Node.create({
  name: 'editorialBlock',
  group: 'block',
  content: 'block+',
  defining: true,
  addAttributes() {
    return {
      kind: {
        default: 'callout',
        parseHTML: element => {
          const kind = element.getAttribute('data-editorial-block')
          return editorialBlockKinds.includes(kind as EditorialBlockKind) ? kind : 'callout'
        },
        renderHTML: attributes => ({ 'data-editorial-block': editorialBlockKinds.includes(attributes.kind) ? attributes.kind : 'callout' })
      }
    }
  },
  parseHTML: () => [{ tag: 'aside[data-editorial-block]' }],
  renderHTML: ({ HTMLAttributes }) => ['aside', mergeAttributes(HTMLAttributes), 0]
})

const SemanticTable = Table.extend({
  addAttributes() {
    return {
      ...(this.parent?.() || {}),
      comparison: {
        default: false,
        parseHTML: element => element.getAttribute('data-comparison-table') === 'true',
        renderHTML: attributes => attributes.comparison ? { 'data-comparison-table': 'true' } : {}
      }
    }
  }
}).configure({ resizable: true })

const TaskList = Node.create({
  name: 'taskList',
  group: 'block',
  content: 'taskItem+',
  defining: true,
  parseHTML: () => [{ tag: 'ul[data-task-list]' }],
  renderHTML: ({ HTMLAttributes }) => ['ul', mergeAttributes(HTMLAttributes, { 'data-task-list': 'true' }), 0]
})

const TaskItem = Node.create({
  name: 'taskItem',
  content: 'paragraph block*',
  defining: true,
  addAttributes() {
    return {
      checked: {
        default: false,
        parseHTML: element => element.getAttribute('data-checked') === 'true',
        renderHTML: attributes => ({ 'data-checked': attributes.checked ? 'true' : 'false' })
      }
    }
  },
  parseHTML: () => [{ tag: 'li[data-checked]' }],
  renderHTML: ({ HTMLAttributes }) => ['li', mergeAttributes(HTMLAttributes), 0]
})

const AttributedQuote = Node.create({
  name: 'attributedQuote',
  group: 'block',
  content: 'blockquote figcaption?',
  defining: true,
  parseHTML: () => [{ tag: 'figure[data-attributed-quote]' }],
  renderHTML: ({ HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes, { 'data-attributed-quote': 'true' }), 0]
})

const Gallery = Node.create({
  name: 'gallery',
  group: 'block',
  content: 'figure+',
  defining: true,
  parseHTML: () => [{ tag: 'div[data-gallery]' }],
  renderHTML: ({ HTMLAttributes }) => ['div', mergeAttributes(HTMLAttributes, { 'data-gallery': 'true' }), 0]
})

const Transcript = Node.create({
  name: 'transcript',
  group: 'block',
  content: 'block+',
  defining: true,
  parseHTML: () => [{ tag: 'section[data-transcript]' }],
  renderHTML: ({ HTMLAttributes }) => ['section', mergeAttributes(HTMLAttributes, { 'data-transcript': 'true' }), 0]
})

const RelatedReference = Node.create({
  name: 'relatedReference',
  group: 'block',
  content: 'block+',
  defining: true,
  addAttributes() {
    return {
      articleId: {
        default: null,
        parseHTML: element => element.getAttribute('data-related-article-id'),
        renderHTML: attributes => safeReferenceID(attributes.articleId) ? { 'data-related-article-id': attributes.articleId } : {}
      }
    }
  },
  parseHTML: () => [{ tag: 'aside[data-related-reference]' }],
  renderHTML: ({ HTMLAttributes }) => ['aside', mergeAttributes(HTMLAttributes, { 'data-related-reference': 'true' }), 0]
})

const EmbedReference = Node.create({
  name: 'embedReference',
  group: 'block',
  content: 'paragraph+',
  defining: true,
  addAttributes() {
    return {
      provider: {
        default: 'youtube',
        parseHTML: element => safeEmbedProvider(element.getAttribute('data-embed-provider') || '') ? element.getAttribute('data-embed-provider') : 'youtube',
        renderHTML: attributes => ({ 'data-embed-provider': safeEmbedProvider(attributes.provider) ? attributes.provider : 'youtube' })
      },
      url: {
        default: null,
        parseHTML: element => element.getAttribute('data-embed-url'),
        renderHTML: attributes => isSafeEmbedURL(attributes.url, attributes.provider) ? { 'data-embed-url': attributes.url } : {}
      }
    }
  },
  parseHTML: () => [{ tag: 'figure[data-embed-provider][data-embed-url]' }],
  renderHTML: ({ HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes), 0]
})

const Citation = Node.create({
  name: 'citation',
  group: 'inline',
  inline: true,
  content: 'text*',
  atom: true,
  addAttributes() {
    return {
      sourceId: {
        default: null,
        parseHTML: element => element.getAttribute('data-source-id'),
        renderHTML: attributes => safeReferenceID(attributes.sourceId) ? { 'data-source-id': attributes.sourceId } : {}
      },
      href: {
        default: null,
        parseHTML: element => element.querySelector('a')?.getAttribute('href') || null,
        rendered: false
      }
    }
  },
  parseHTML: () => [{ tag: 'cite[data-source-id]' }],
  renderHTML: ({ node, HTMLAttributes }) => {
    const href = isSafeEditorialURL(node.attrs.href || '', true) ? node.attrs.href : ''
    return ['cite', mergeAttributes(HTMLAttributes), href ? ['a', { href }, 0] : 0]
  }
})

function semanticMark(name: string, tag: string) {
  return Mark.create({
    name,
    parseHTML: () => [{ tag }],
    renderHTML: ({ HTMLAttributes }) => [tag, mergeAttributes(HTMLAttributes), 0]
  })
}

const Superscript = semanticMark('superscript', 'sup')
const Subscript = semanticMark('subscript', 'sub')

const activeTextStyle = computed(() => {
  if (!editor.value) return 'paragraph'
  for (const level of [2, 3, 4] as const) {
    if (editor.value.isActive('heading', { level })) return `heading-${level}`
  }
  return 'paragraph'
})

const dialogTitle = computed(() => ({
  link: 'Add a link',
  image: 'Insert an image',
  anchor: 'Edit heading anchor',
  related: 'Insert a related article',
  embed: 'Insert an embed'
}[editorDialog.value || 'link']))

const dialogDescription = computed(() => ({
  link: 'Link the selected text to a safe URL.',
  image: 'Use an HTTPS or root-relative image URL.',
  anchor: 'Create a stable link directly to this heading.',
  related: 'Reference another article by its ID.',
  embed: 'Add a supported video or media reference.'
}[editorDialog.value || 'link']))

const dialogActionLabel = computed(() => editorDialog.value === 'link' && editor.value?.isActive('link') ? 'Update link' : 'Insert')

onMounted(() => {
  const instance = new Editor({
    editable: !props.disabled,
    extensions: [
      StarterKit.configure({
        heading: false,
        link: {
          openOnClick: false,
          autolink: false,
          linkOnPaste: false,
          defaultProtocol: 'https',
          isAllowedUri: url => isSafeEditorialURL(url, true)
        }
      }),
      StableHeading,
      AccessibleImage,
      Figure,
      Figcaption,
      EditorialBlock,
      TaskList,
      TaskItem,
      AttributedQuote,
      Gallery,
      Transcript,
      RelatedReference,
      EmbedReference,
      Citation,
      Superscript,
      Subscript,
      SemanticTable,
      TableRow,
      TableHeader,
      TableCell
    ],
    content: initialContent(),
    onCreate: ({ editor: createdEditor }) => {
      createdEditor.view.dispatch(createdEditor.state.tr.setMeta(normalizeHeadingIDsKey, true))
      emitEditorState(createdEditor)
    },
    onUpdate: ({ editor: updatedEditor }) => emitEditorState(updatedEditor)
  })
  editor.value = instance
  document.addEventListener('pointerdown', closeMenusOnOutsideClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', closeMenusOnOutsideClick)
  editor.value?.destroy()
  editor.value = null
})

watch(() => props.disabled, value => editor.value?.setEditable(!value))

watch(() => props.html, (value) => {
  if (!editor.value || normalizeHTML(value) === normalizeHTML(editor.value.getHTML())) return
  editor.value.commands.setContent(value.trim() || '<p></p>', { emitUpdate: false })
})

function initialContent() {
  if (isStructuredDocument(props.bodyDocument)) return props.bodyDocument
  return props.html.trim() || '<p></p>'
}

function emitEditorState(instance: Pick<Editor, 'getJSON' | 'getHTML'>) {
  const document = instance.getJSON() as Record<string, unknown>
  emit('update:html', instance.getHTML())
  emit('update:bodyDocument', { ...document, schemaVersion: 'tiptap-v1' })
}

function setTextStyle(value: string) {
  if (!editor.value) return
  if (value === 'paragraph') {
    editor.value.chain().focus().setParagraph().run()
    return
  }
  const level = Number(value.replace('heading-', '')) as 2 | 3 | 4
  if ([2, 3, 4].includes(level)) editor.value.chain().focus().setHeading({ level }).run()
}

function run(command: 'toggleBold' | 'toggleItalic' | 'toggleUnderline' | 'toggleStrike' | 'toggleCode' | 'toggleBulletList' | 'toggleOrderedList' | 'toggleBlockquote' | 'toggleCodeBlock') {
  const instance = editor.value
  if (!instance) return
  instance.chain().focus()[command]().run()
}

function buttonClass(format: string) {
  return ['structured-editor__button', { 'is-active': editor.value?.isActive(format) }]
}

function insertFromMenu(action: () => unknown, menu = insertMenu.value) {
  if (props.disabled) return
  action()
  if (menu) menu.open = false
}

function insertEditorialFromMenu(kind: EditorialBlockKind) {
  if (props.disabled) return
  insertEditorialBlock(kind)
  if (insertMenu.value) insertMenu.value.open = false
}

function closeMenusOnOutsideClick(event: PointerEvent) {
  const target = event.target as globalThis.Node | null
  if (target && (insertMenu.value?.contains(target) || moreMenu.value?.contains(target))) return
  if (insertMenu.value) insertMenu.value.open = false
  if (moreMenu.value) moreMenu.value.open = false
}

function openEditorDialog(kind: EditorDialog, values: Partial<typeof dialogFields> = {}) {
  Object.assign(dialogFields, { url: '', alt: '', caption: '', anchor: '', articleId: '', provider: 'youtube' }, values)
  dialogError.value = ''
  editorError.value = ''
  editorDialog.value = kind
}

function closeEditorDialog() {
  editorDialog.value = null
  dialogError.value = ''
  editor.value?.commands.focus()
}

function removeLink() {
  editor.value?.chain().focus().extendMarkRange('link').unsetLink().run()
  closeEditorDialog()
}

function applyEditorDialog() {
  const instance = editor.value
  const kind = editorDialog.value
  if (!instance || !kind) return
  dialogError.value = ''

  if (kind === 'link') {
    const href = dialogFields.url.trim()
    if (!isSafeEditorialURL(href, true)) {
      dialogError.value = 'Use an HTTPS, mailto, root-relative, or document-anchor link.'
      return
    }
    instance.chain().focus().extendMarkRange('link').setLink({ href }).run()
  }

  if (kind === 'image') {
    const src = dialogFields.url.trim()
    if (!isSafeEditorialURL(src, false)) {
      dialogError.value = 'Use an HTTPS or root-relative image URL.'
      return
    }
    const alt = dialogFields.alt.trim()
    const caption = dialogFields.caption.trim()
    const image = { type: 'image', attrs: { src, alt, decorative: alt.length === 0 } }
    instance.chain().focus().insertContent(caption
      ? { type: 'figure', content: [image, { type: 'figcaption', content: [{ type: 'text', text: caption }] }] }
      : image).run()
  }

  if (kind === 'anchor') {
    const id = dialogFields.anchor.trim()
    if (!safeHeadingID(id)) {
      dialogError.value = 'Start with a letter and use only letters, numbers, hyphens, or underscores.'
      return
    }
    const currentPosition = instance.state.selection.$from.before(instance.state.selection.$from.depth)
    let conflict = false
    instance.state.doc.descendants((node, position) => {
      if (node.type.name === 'heading' && position !== currentPosition && node.attrs.id === id) conflict = true
    })
    if (conflict) {
      dialogError.value = `The anchor “${id}” is already used by another heading.`
      return
    }
    instance.chain().focus().updateAttributes('heading', { id }).run()
  }

  if (kind === 'related') {
    const articleId = dialogFields.articleId.trim()
    if (!safeReferenceID(articleId)) {
      dialogError.value = 'Use only letters, numbers, hyphens, and underscores.'
      return
    }
    instance.chain().focus().insertContent({
      type: 'relatedReference',
      attrs: { articleId },
      content: [
        { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: 'Related reading' }] },
        { type: 'paragraph', content: [{ type: 'text', text: 'Summarize why this article is relevant.' }] }
      ]
    }).run()
  }

  if (kind === 'embed') {
    const provider = dialogFields.provider.trim().toLowerCase()
    const url = dialogFields.url.trim()
    if (!safeEmbedProvider(provider) || !isSafeEmbedURL(url, provider)) {
      dialogError.value = 'Use an HTTPS URL from the selected provider.'
      return
    }
    instance.chain().focus().insertContent({
      type: 'embedReference',
      attrs: { provider, url },
      content: [{ type: 'paragraph', content: [{ type: 'text', text: `${providerLabel(provider)} embed` }] }]
    }).run()
  }

  closeEditorDialog()
}

function editLink() {
  if (!editor.value) return
  openEditorDialog('link', { url: String(editor.value.getAttributes('link').href || '') })
}

function editHeadingAnchor() {
  if (!editor.value?.isActive('heading')) return
  openEditorDialog('anchor', { anchor: String(editor.value.getAttributes('heading').id || '') })
}

function insertImage() {
  if (!editor.value) return
  openEditorDialog('image')
}

function insertEditorialBlock(kind: EditorialBlockKind) {
  if (!editor.value) return
  const labels: Record<EditorialBlockKind, string> = {
    callout: 'Callout',
    takeaway: 'Key takeaway',
    steps: 'Steps',
    'pros-cons': 'Pros and cons',
    cta: 'Next step',
    faq: 'Frequently asked question'
  }
  editor.value.chain().focus().insertContent({
    type: 'editorialBlock',
    attrs: { kind },
    content: [
      { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: labels[kind] }] },
      { type: 'paragraph', content: [{ type: 'text', text: 'Add editorial content.' }] }
    ]
  }).run()
}

function insertTaskList() {
  editor.value?.chain().focus().insertContent({
    type: 'taskList',
    content: [
      taskItem('Confirm source evidence', false),
      taskItem('Add subject expert notes', false),
      taskItem('Prepare publishing checks', false)
    ]
  }).run()
}

function taskItem(text: string, checked: boolean) {
  return {
    type: 'taskItem',
    attrs: { checked },
    content: [{ type: 'paragraph', content: [{ type: 'text', text }] }]
  }
}

function insertAttributedQuote() {
  editor.value?.chain().focus().insertContent({
    type: 'attributedQuote',
    content: [
      { type: 'blockquote', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Add the exact quote.' }] }] },
      { type: 'figcaption', content: [{ type: 'text', text: 'Attribution, role or source' }] }
    ]
  }).run()
}

function insertComparisonTable() {
  editor.value?.chain().focus().insertContent({
    type: 'table',
    attrs: { comparison: true },
    content: [
      tableRow('tableHeader', ['Criteria', 'Option A', 'Option B']),
      tableRow('tableCell', ['Best for', 'Add details', 'Add details']),
      tableRow('tableCell', ['Tradeoff', 'Add details', 'Add details'])
    ]
  }).run()
}

function tableRow(cellType: 'tableHeader' | 'tableCell', labels: string[]) {
  return {
    type: 'tableRow',
    content: labels.map(label => ({
      type: cellType,
      attrs: cellType === 'tableHeader' ? { scope: 'col' } : {},
      content: [{ type: 'paragraph', content: [{ type: 'text', text: label }] }]
    }))
  }
}

function insertGallery() {
  const asset = insertableMedia.value[0]
  const image = asset?.url
    ? { src: asset.url, alt: asset.altText || 'Gallery image', decorative: asset.decorative }
    : { src: '/media/gallery-placeholder.jpg', alt: 'Gallery image', decorative: false }
  editor.value?.chain().focus().insertContent({
    type: 'gallery',
    content: [
      galleryFigure(image.src, image.alt, image.decorative, asset?.caption || 'Gallery caption'),
      galleryFigure(image.src, image.alt, image.decorative, 'Second gallery caption')
    ]
  }).run()
}

function galleryFigure(src: string, alt: string, decorative: boolean, caption: string) {
  return {
    type: 'figure',
    content: [
      { type: 'image', attrs: { src, alt, decorative } },
      { type: 'figcaption', content: [{ type: 'text', text: caption }] }
    ]
  }
}

function insertTranscript() {
  editor.value?.chain().focus().insertContent({
    type: 'transcript',
    content: [
      { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: 'Transcript' }] },
      { type: 'paragraph', content: [{ type: 'text', text: 'Speaker: Add timestamped transcript text.' }] }
    ]
  }).run()
}

function insertRelatedReference() {
  if (!editor.value) return
  openEditorDialog('related')
}

function insertEmbed() {
  if (!editor.value) return
  openEditorDialog('embed')
}

function insertSelectedMedia() {
  const asset = insertableMedia.value.find(candidate => candidate.id === selectedMediaID.value)
  if (!editor.value || !asset?.url) return
  editorError.value = ''
  const alt = (asset.altText || '').trim()
  if (!alt && !asset.decorative) {
    editorError.value = 'This project image needs alt text in the media library before it can be inserted.'
    return
  }
  const image = { type: 'image', attrs: { src: asset.url, alt, decorative: asset.decorative } }
  const caption = (asset.caption || '').trim()
  editor.value.chain().focus().insertContent(caption
    ? { type: 'figure', content: [image, { type: 'figcaption', content: [{ type: 'text', text: caption }] }] }
    : image).run()
  selectedMediaID.value = ''
}

function insertSelectedCitation() {
  const source = props.sources.find(candidate => candidate.id === selectedSourceID.value)
  if (!editor.value || !source || !safeReferenceID(source.id)) return
  editorError.value = ''
  const href = source.url && isSafeEditorialURL(source.url, true) ? source.url : null
  editor.value.chain().focus().insertContent({
    type: 'citation',
    attrs: { sourceId: source.id, href },
    content: [{ type: 'text', text: source.title }]
  }).run()
  selectedSourceID.value = ''
}

function mediaLabel(asset: AdminMediaAsset) {
  const dimensions = asset.width && asset.height ? ` · ${asset.width}×${asset.height}` : ''
  return `${asset.filename}${dimensions}`
}

function isStructuredDocument(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && (value as Record<string, unknown>).type === 'doc' && Array.isArray((value as Record<string, unknown>).content))
}

function normalizeHTML(value: string) {
  return value.trim().replace(/>\s+</g, '><')
}

function safeHeadingID(value: string) {
  return /^[A-Za-z][A-Za-z0-9_-]{0,127}$/.test(value)
}

function safeReferenceID(value: unknown): value is string {
  return typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(value)
}

function headingIDFromText(value: string) {
  let candidate = value
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 96)
  if (!candidate) candidate = 'section'
  if (!/^[A-Za-z]/.test(candidate)) candidate = `section-${candidate}`
  return candidate
}

function uniqueHeadingID(base: string, used: Set<string>) {
  let candidate = base
  let suffix = 2
  while (used.has(candidate)) {
    const suffixText = `-${suffix}`
    candidate = `${base.slice(0, 128 - suffixText.length)}${suffixText}`
    suffix += 1
  }
  used.add(candidate)
  return candidate
}

function isSafeEditorialURL(raw: string, allowLinkSchemes: boolean) {
  if (!raw || /[\u0000\r\n\\]/.test(raw)) return false
  if (allowLinkSchemes && raw.startsWith('#')) return true
  if (raw.startsWith('/') && !raw.startsWith('//')) return true
  try {
    const parsed = new URL(raw)
    if (parsed.username || parsed.password) return false
    if (allowLinkSchemes && parsed.protocol === 'mailto:') return Boolean(parsed.pathname) && !/[<>\"]/.test(parsed.pathname)
    return parsed.protocol === 'https:' && Boolean(parsed.hostname)
  } catch {
    return false
  }
}

function safeEmbedProvider(value: unknown): value is string {
  return typeof value === 'string' && ['youtube', 'vimeo', 'wistia'].includes(value)
}

function isSafeEmbedURL(raw: unknown, provider: unknown) {
  if (typeof raw !== 'string' || typeof provider !== 'string' || !safeEmbedProvider(provider) || !isSafeEditorialURL(raw, false)) return false
  try {
    const host = new URL(raw).hostname.toLowerCase()
    if (provider === 'youtube') return ['youtube.com', 'www.youtube.com', 'youtu.be', 'www.youtu.be'].includes(host)
    if (provider === 'vimeo') return ['vimeo.com', 'www.vimeo.com', 'player.vimeo.com'].includes(host)
    return ['wistia.com', 'www.wistia.com', 'fast.wistia.com'].includes(host)
  } catch {
    return false
  }
}

function providerLabel(provider: string) {
  return provider === 'youtube' ? 'YouTube' : provider === 'vimeo' ? 'Vimeo' : 'Wistia'
}
</script>

<style scoped>
.structured-editor { position: relative; overflow: visible; border: 1px solid var(--border, #bfcac3); border-radius: 8px; background: var(--surface, #fff); box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.7); }
.structured-editor:focus-within { border-color: color-mix(in srgb, var(--primary, #165a4a) 70%, white); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary, #165a4a) 14%, transparent); }
.structured-editor__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 13px 16px; border-bottom: 1px solid var(--border, #d7ded8); border-radius: 8px 8px 0 0; background: linear-gradient(180deg, var(--surface-subtle, #f5f7f5), var(--surface, #fff)); }
.structured-editor__heading > div { display: grid; gap: 2px; }
.structured-editor__label { font-size: 13px; font-weight: 700; }
.structured-editor__heading small,
.structured-editor__help { color: var(--text-faint, #667169); font-size: 12px; }
.structured-editor__mode { padding: 4px 8px; border: 1px solid color-mix(in srgb, var(--primary, #165a4a) 16%, transparent); border-radius: 999px; background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); font-size: 11px; font-weight: 700; letter-spacing: 0; text-transform: uppercase; }
.structured-editor__toolbar { position: sticky; z-index: 4; top: 145px; display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 4px; padding: 9px 10px; border-bottom: 1px solid var(--border, #d7ded8); background: color-mix(in srgb, var(--surface, #fff) 96%, transparent); backdrop-filter: blur(10px); }
.structured-editor__group { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 2px; padding-left: 5px; margin-left: 2px; border-left: 1px solid var(--border, #d7ded8); }
.structured-editor__group--history { margin-left: 4px; }
.structured-editor__spacer { flex: 1 1 auto; }
.structured-editor__select,
.structured-editor__button { min-height: 34px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: var(--text, #28342d); font-size: 13px; }
.structured-editor__select { min-width: 124px; max-width: 220px; min-height: 34px; padding: 0 28px 0 9px; border-color: var(--border-strong, #c9d4cc); background: var(--surface, #fff); }
.structured-editor__button { display: inline-grid; min-width: 34px; padding: 0; place-items: center; cursor: pointer; }
.structured-editor__button:hover:not(:disabled),
.structured-editor__button.is-active { background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary, #165a4a) 14%, transparent); }
.structured-editor__button--danger { color: #9b2d23; }
.structured-editor__button:disabled,
.structured-editor__select:disabled { opacity: .45; cursor: not-allowed; }
.structured-editor__menu { position: relative; flex: 0 0 auto; }
.structured-editor__menu[open] { z-index: 5; }
.structured-editor__menu.is-disabled { opacity: .45; pointer-events: none; }
.structured-editor__menu summary { list-style: none; }
.structured-editor__menu summary::-webkit-details-marker { display: none; }
.structured-editor__menu-trigger { display: inline-flex; min-height: 34px; align-items: center; gap: 5px; padding: 0 9px; border: 1px solid transparent; border-radius: 6px; color: var(--text, #28342d); font-size: 13px; font-weight: 600; cursor: pointer; user-select: none; }
.structured-editor__menu-trigger:hover,
.structured-editor__menu[open] > .structured-editor__menu-trigger { background: var(--surface-subtle, #f2f5f3); color: var(--primary, #165a4a); }
.structured-editor__menu-panel { position: absolute; top: calc(100% + 7px); left: 0; display: grid; width: 286px; max-height: 440px; overflow-y: auto; padding: 7px; border: 1px solid var(--border, #d7ded8); border-radius: 8px; background: var(--surface, #fff); box-shadow: 0 18px 45px rgb(15 23 42 / 16%); }
.structured-editor__menu-panel--right { right: 0; left: auto; }
.structured-editor__menu-label { margin: 7px 8px 4px; color: var(--text-faint, #667169); font-size: 10px; font-weight: 800; letter-spacing: 0; text-transform: uppercase; }
.structured-editor__menu-panel button { display: grid; grid-template-columns: 22px 1fr; gap: 9px; align-items: center; width: 100%; padding: 8px; border: 0; border-radius: 6px; background: transparent; color: var(--text, #28342d); text-align: left; cursor: pointer; }
.structured-editor__menu-panel button:hover { background: var(--surface-subtle, #f2f5f3); color: var(--primary, #165a4a); }
.structured-editor__menu-panel button > span { display: grid; gap: 1px; }
.structured-editor__menu-panel strong { font-size: 13px; font-weight: 650; }
.structured-editor__menu-panel small { color: var(--text-faint, #667169); font-size: 11px; }
.structured-editor__context-bar { position: relative; z-index: 3; display: flex; flex-wrap: wrap; gap: 8px 16px; align-items: center; padding: 8px 10px; border-bottom: 1px solid var(--border, #d7ded8); background: var(--surface-subtle, #f5f7f5); }
.structured-editor__picker { display: inline-flex; min-width: 0; align-items: center; gap: 6px; color: var(--text-faint, #667169); }
.structured-editor__picker .structured-editor__select { min-width: 150px; min-height: 30px; font-size: 12px; }
.structured-editor__picker--context { margin-left: auto; }
.structured-editor__context-label { font-size: 12px; font-weight: 700; }
.structured-editor__text-button { display: inline-flex; min-height: 28px; align-items: center; gap: 4px; padding: 0 7px; border: 0; border-radius: 5px; background: transparent; color: var(--primary, #165a4a); font-size: 12px; font-weight: 700; cursor: pointer; }
.structured-editor__text-button:hover:not(:disabled) { background: var(--primary-soft, #e6f2ec); }
.structured-editor__text-button:disabled { opacity: .45; cursor: not-allowed; }
.structured-editor__text-button--danger { color: #9b2d23; }
.structured-editor__dialog { position: relative; z-index: 2; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 14px; border-bottom: 1px solid var(--border, #d7ded8); background: color-mix(in srgb, var(--primary-soft, #e6f2ec) 48%, var(--surface, #fff)); }
.structured-editor__dialog-heading { grid-column: 1 / -1; display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.structured-editor__dialog-heading > div { display: grid; gap: 2px; }
.structured-editor__dialog-heading strong { font-size: 13px; }
.structured-editor__dialog-heading small { color: var(--text-faint, #667169); font-size: 11px; }
.structured-editor__dialog-heading > button { display: grid; width: 28px; height: 28px; place-items: center; border: 0; border-radius: 5px; background: transparent; color: var(--text-faint, #667169); cursor: pointer; }
.structured-editor__dialog-heading > button:hover { background: var(--surface-subtle, #f2f5f3); color: var(--text, #28342d); }
.structured-editor__dialog label { display: grid; gap: 5px; min-width: 0; color: var(--text, #28342d); font-size: 12px; font-weight: 650; }
.structured-editor__dialog label > span small { color: var(--text-faint, #667169); font-weight: 400; }
.structured-editor__dialog input,
.structured-editor__dialog select { width: 100%; min-height: 36px; padding: 0 10px; border: 1px solid var(--border, #bfcac3); border-radius: 6px; background: var(--surface, #fff); color: var(--text, #28342d); font-size: 13px; outline: none; }
.structured-editor__dialog input:focus,
.structured-editor__dialog select:focus { border-color: var(--primary, #165a4a); box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary, #165a4a) 12%, transparent); }
.structured-editor__input-prefix { display: flex; align-items: center; border: 1px solid var(--border, #bfcac3); border-radius: 6px; background: var(--surface, #fff); }
.structured-editor__input-prefix > span { padding-left: 10px; color: var(--text-faint, #667169); }
.structured-editor__input-prefix input { border: 0; box-shadow: none; }
.structured-editor__dialog-error { grid-column: 1 / -1; margin: 0; color: #9b2d23; font-size: 12px; }
.structured-editor__dialog-actions { grid-column: 1 / -1; display: flex; justify-content: flex-end; gap: 7px; }
.structured-editor__dialog-actions button { min-height: 32px; padding: 0 11px; border: 1px solid var(--border, #bfcac3); border-radius: 6px; background: var(--surface, #fff); color: var(--text, #28342d); font-size: 12px; font-weight: 700; cursor: pointer; }
.structured-editor__dialog-actions .structured-editor__dialog-remove { margin-right: auto; border-color: transparent; background: transparent; color: #9b2d23; }
.structured-editor__dialog-actions .structured-editor__dialog-primary { border-color: var(--primary, #165a4a); background: var(--primary, #165a4a); color: white; }
.structured-editor__canvas { position: relative; overflow: hidden; border-radius: 0 0 8px 8px; background: color-mix(in srgb, var(--surface, #fff) 96%, var(--surface-subtle, #f5f7f5)); }
.structured-editor__loading { display: grid; min-height: 260px; place-items: center; color: var(--text-faint, #667169); font-size: 13px; }
.structured-editor__error { margin: 0; padding: 8px 12px; border-top: 1px solid #edc6c2; background: #fff4f2; color: #9b2d23; font-size: 12px; }
.structured-editor__help { margin: 0; padding: 8px 14px; border-top: 1px solid var(--border, #d7ded8); border-radius: 0 0 8px 8px; background: var(--surface-subtle, #f5f7f5); }
.structured-editor--disabled { opacity: .75; }
:deep(.tiptap) { min-height: 470px; max-width: 900px; margin: 0 auto; padding: 34px 56px; color: var(--text, #28342d); font-size: 16px; line-height: 1.75; outline: none; }
:deep(.tiptap > :first-child) { margin-top: 0; }
:deep(.tiptap > :last-child) { margin-bottom: 0; }
:deep(.tiptap p),
:deep(.tiptap ul),
:deep(.tiptap ol),
:deep(.tiptap blockquote),
:deep(.tiptap pre),
:deep(.tiptap table),
:deep(.tiptap figure),
:deep(.tiptap aside[data-editorial-block]),
:deep(.tiptap ul[data-task-list]),
:deep(.tiptap figure[data-attributed-quote]),
:deep(.tiptap div[data-gallery]),
:deep(.tiptap section[data-transcript]),
:deep(.tiptap aside[data-related-reference]),
:deep(.tiptap figure[data-embed-provider]) { margin: 0 0 12px; }
:deep(.tiptap h2),
:deep(.tiptap h3),
:deep(.tiptap h4) { margin: 20px 0 8px; font-weight: 700; line-height: 1.25; }
:deep(.tiptap h2) { font-size: 22px; }
:deep(.tiptap h3) { font-size: 18px; }
:deep(.tiptap h4) { font-size: 15px; }
:deep(.tiptap ul),
:deep(.tiptap ol) { padding-left: 24px; }
:deep(.tiptap blockquote) { padding-left: 12px; border-left: 3px solid var(--border-strong, #9faea4); color: var(--text-soft, #4f5b54); }
:deep(.tiptap pre) { overflow-x: auto; padding: 12px; border-radius: 6px; background: #171b18; color: #eef4ef; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
:deep(.tiptap code) { padding: 1px 3px; border-radius: 3px; background: var(--surface-subtle, #f2f5f3); font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
:deep(.tiptap pre code) { padding: 0; background: transparent; }
:deep(.tiptap a) { color: var(--primary, #165a4a); text-decoration: underline; }
:deep(.tiptap cite[data-source-id]) { padding: 1px 4px; border-radius: 3px; background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); font-style: normal; }
:deep(.tiptap img) { max-width: 100%; height: auto; border-radius: 6px; }
:deep(.tiptap figure) { padding: 8px; border: 1px solid var(--border, #d7ded8); border-radius: 6px; }
:deep(.tiptap figcaption) { margin-top: 6px; color: var(--text-faint, #667169); font-size: 12px; }
:deep(.tiptap aside[data-editorial-block]) { padding: 14px; border: 1px solid var(--border, #c9d4cc); border-left: 4px solid var(--primary, #165a4a); border-radius: 6px; background: var(--surface-subtle, #f2f5f3); }
:deep(.tiptap aside[data-editorial-block='takeaway']) { border-left-color: #1d6c9f; }
:deep(.tiptap aside[data-editorial-block='cta']) { border-left-color: #9b5a18; }
:deep(.tiptap ul[data-task-list]) { padding-left: 0; list-style: none; }
:deep(.tiptap li[data-checked]) { position: relative; margin-bottom: 8px; padding-left: 26px; }
:deep(.tiptap li[data-checked]::before) { position: absolute; top: 4px; left: 0; display: grid; width: 16px; height: 16px; place-items: center; border: 1px solid var(--border-strong, #9faea4); border-radius: 4px; color: white; font-size: 11px; line-height: 1; content: ''; }
:deep(.tiptap li[data-checked='true']::before) { border-color: var(--primary, #165a4a); background: var(--primary, #165a4a); content: '✓'; }
:deep(.tiptap li[data-checked] > :last-child) { margin-bottom: 0; }
:deep(.tiptap figure[data-attributed-quote]) { border-left: 4px solid #6b5797; background: color-mix(in srgb, #6b5797 8%, var(--surface, #fff)); }
:deep(.tiptap table[data-comparison-table]) { border: 2px solid color-mix(in srgb, var(--primary, #165a4a) 40%, var(--border, #d7ded8)); }
:deep(.tiptap div[data-gallery]) { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; padding: 10px; border: 1px solid var(--border, #d7ded8); border-radius: 6px; }
:deep(.tiptap div[data-gallery] figure) { margin: 0; }
:deep(.tiptap section[data-transcript]) { padding: 12px; border: 1px dashed var(--border-strong, #9faea4); border-radius: 6px; background: var(--surface-subtle, #f2f5f3); }
:deep(.tiptap aside[data-related-reference]) { padding: 12px; border: 1px solid #b9cde8; border-left: 4px solid #3162a3; border-radius: 6px; background: #f2f7ff; }
:deep(.tiptap figure[data-embed-provider]) { padding: 12px; border: 1px solid #d8d0e8; border-left: 4px solid #6b5797; background: #f7f4fc; }
:deep(.tiptap .tableWrapper) { overflow-x: auto; margin-bottom: 12px; }
:deep(.tiptap table) { width: 100%; border-collapse: collapse; table-layout: fixed; }
:deep(.tiptap th),
:deep(.tiptap td) { min-width: 90px; padding: 7px 8px; border: 1px solid var(--border-strong, #9faea4); vertical-align: top; }
:deep(.tiptap th) { background: var(--surface-subtle, #f2f5f3); font-weight: 700; }
:deep(.tiptap .selectedCell::after) { position: absolute; inset: 0; z-index: 2; background: color-mix(in srgb, var(--primary, #165a4a) 15%, transparent); content: ''; pointer-events: none; }
@media (max-width: 680px) {
  .structured-editor__toolbar > .structured-editor__select { min-width: 112px; }
  .structured-editor__spacer { display: none; }
  .structured-editor__menu-panel--right { right: auto; left: 0; }
  .structured-editor__context-bar { align-items: stretch; }
  .structured-editor__picker { width: 100%; }
  .structured-editor__picker .structured-editor__select { flex: 1; }
  .structured-editor__picker--context { margin-left: 0; }
  .structured-editor__dialog { grid-template-columns: 1fr; }
  .structured-editor__heading { align-items: flex-start; }
  .structured-editor__mode { display: none; }
  :deep(.tiptap) { min-height: 340px; padding: 20px 16px; }
}
</style>
