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

      <span class="structured-editor__group" aria-label="Inline formatting">
        <button type="button" :class="buttonClass('bold')" :aria-pressed="editor.isActive('bold')" :disabled="disabled" title="Bold" @click="run('toggleBold')"><strong>B</strong></button>
        <button type="button" :class="buttonClass('italic')" :aria-pressed="editor.isActive('italic')" :disabled="disabled" title="Italic" @click="run('toggleItalic')"><em>I</em></button>
        <button type="button" :class="buttonClass('underline')" :aria-pressed="editor.isActive('underline')" :disabled="disabled" title="Underline" @click="run('toggleUnderline')"><u>U</u></button>
        <button type="button" :class="buttonClass('strike')" :aria-pressed="editor.isActive('strike')" :disabled="disabled" title="Strikethrough" @click="run('toggleStrike')"><s>S</s></button>
        <button type="button" :class="buttonClass('code')" :aria-pressed="editor.isActive('code')" :disabled="disabled" title="Inline code" @click="run('toggleCode')"><code>&lt;/&gt;</code></button>
      </span>

      <span class="structured-editor__group" aria-label="Links">
        <button type="button" :class="buttonClass('link')" :aria-pressed="editor.isActive('link')" :disabled="disabled" title="Add or edit link" @click="editLink">Link</button>
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.isActive('link')" title="Remove link" @click="editor.chain().focus().unsetLink().run()">Unlink</button>
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.isActive('heading')" title="Set this heading's anchor" @click="editHeadingAnchor">Anchor</button>
      </span>

      <span class="structured-editor__group" aria-label="Blocks">
        <button type="button" :class="buttonClass('bulletList')" :aria-pressed="editor.isActive('bulletList')" :disabled="disabled" title="Bulleted list" @click="run('toggleBulletList')">Bullets</button>
        <button type="button" :class="buttonClass('orderedList')" :aria-pressed="editor.isActive('orderedList')" :disabled="disabled" title="Numbered list" @click="run('toggleOrderedList')">Numbers</button>
        <button type="button" :class="buttonClass('blockquote')" :aria-pressed="editor.isActive('blockquote')" :disabled="disabled" title="Block quote" @click="run('toggleBlockquote')">Quote</button>
        <button type="button" :class="buttonClass('codeBlock')" :aria-pressed="editor.isActive('codeBlock')" :disabled="disabled" title="Code block" @click="run('toggleCodeBlock')">Code block</button>
      </span>

      <span class="structured-editor__group" aria-label="Insert content">
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert horizontal rule" @click="editor.chain().focus().setHorizontalRule().run()">Rule</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a 3 by 3 table" @click="editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()">Table</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert image from URL" @click="insertImage">Image</button>
      </span>

      <span v-if="mediaAssets.length" class="structured-editor__group structured-editor__picker" aria-label="Project media">
        <select v-model="selectedMediaID" class="structured-editor__select" aria-label="Project image" :disabled="disabled">
          <option value="">Project image…</option>
          <option v-for="asset in insertableMedia" :key="asset.id" :value="asset.id">{{ mediaLabel(asset) }}</option>
        </select>
        <button type="button" class="structured-editor__button" :disabled="disabled || !selectedMediaID" @click="insertSelectedMedia">Insert</button>
      </span>

      <span v-if="sources.length" class="structured-editor__group structured-editor__picker" aria-label="Project citations">
        <select v-model="selectedSourceID" class="structured-editor__select" aria-label="Project source" :disabled="disabled">
          <option value="">Citation…</option>
          <option v-for="source in sources" :key="source.id" :value="source.id">{{ source.title }}</option>
        </select>
        <button type="button" class="structured-editor__button" :disabled="disabled || !selectedSourceID" @click="insertSelectedCitation">Cite</button>
      </span>

      <span class="structured-editor__group" aria-label="Editorial blocks">
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a callout" @click="insertEditorialBlock('callout')">Callout</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a key takeaway" @click="insertEditorialBlock('takeaway')">Takeaway</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a steps section" @click="insertEditorialBlock('steps')">Steps</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a pros and cons section" @click="insertEditorialBlock('pros-cons')">Pros / cons</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a call to action" @click="insertEditorialBlock('cta')">CTA</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a FAQ section" @click="insertEditorialBlock('faq')">FAQ</button>
      </span>

      <span class="structured-editor__group" aria-label="Specialized blocks">
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a task checklist" @click="insertTaskList">Tasks</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert an attributed quote" @click="insertAttributedQuote">Quote + cite</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a comparison table" @click="insertComparisonTable">Compare</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert an image gallery" @click="insertGallery">Gallery</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a transcript" @click="insertTranscript">Transcript</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert a related article reference" @click="insertRelatedReference">Related</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" title="Insert an allowlisted embed reference" @click="insertEmbed">Embed</button>
      </span>

      <span v-if="editor.isActive('table')" class="structured-editor__group" aria-label="Table controls">
        <button type="button" class="structured-editor__button" :disabled="disabled" @click="editor.chain().focus().addRowAfter().run()">+ Row</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" @click="editor.chain().focus().addColumnAfter().run()">+ Column</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" @click="editor.chain().focus().deleteRow().run()">− Row</button>
        <button type="button" class="structured-editor__button" :disabled="disabled" @click="editor.chain().focus().deleteColumn().run()">− Column</button>
        <button type="button" class="structured-editor__button structured-editor__button--danger" :disabled="disabled" @click="editor.chain().focus().deleteTable().run()">Delete table</button>
      </span>

      <span class="structured-editor__group structured-editor__group--history" aria-label="History">
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.can().chain().focus().undo().run()" title="Undo" @click="editor.chain().focus().undo().run()">Undo</button>
        <button type="button" class="structured-editor__button" :disabled="disabled || !editor.can().chain().focus().redo().run()" title="Redo" @click="editor.chain().focus().redo().run()">Redo</button>
      </span>
    </div>

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
})

onBeforeUnmount(() => {
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

function editLink() {
  if (!editor.value || !import.meta.client) return
  editorError.value = ''
  const current = String(editor.value.getAttributes('link').href || '')
  const requested = window.prompt('Link URL (HTTPS, mailto, /path, or #anchor)', current)
  if (requested === null) return
  const href = requested.trim()
  if (!href) {
    editor.value.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }
  if (!isSafeEditorialURL(href, true)) {
    editorError.value = 'Use an HTTPS, mailto, root-relative, or document-anchor link.'
    return
  }
  editor.value.chain().focus().extendMarkRange('link').setLink({ href }).run()
}

function editHeadingAnchor() {
  if (!editor.value || !import.meta.client || !editor.value.isActive('heading')) return
  editorError.value = ''
  const current = String(editor.value.getAttributes('heading').id || '')
  const requested = window.prompt('Heading anchor (letters, numbers, hyphens, and underscores)', current)
  if (requested === null) return
  const id = requested.trim()
  if (!safeHeadingID(id)) {
    editorError.value = 'Anchors must start with a letter and use only letters, numbers, hyphens, or underscores.'
    return
  }
  const currentPosition = editor.value.state.selection.$from.before(editor.value.state.selection.$from.depth)
  let conflict = false
  editor.value.state.doc.descendants((node, position) => {
    if (node.type.name === 'heading' && position !== currentPosition && node.attrs.id === id) conflict = true
  })
  if (conflict) {
    editorError.value = `The anchor “${id}” is already used by another heading.`
    return
  }
  editor.value.chain().focus().updateAttributes('heading', { id }).run()
}

function insertImage() {
  if (!editor.value || !import.meta.client) return
  editorError.value = ''
  const requested = window.prompt('Image URL (HTTPS or /root-relative path)')
  if (requested === null) return
  const src = requested.trim()
  if (!isSafeEditorialURL(src, false)) {
    editorError.value = 'Use an HTTPS or root-relative image URL.'
    return
  }
  const requestedAlt = window.prompt('Image alt text (leave blank only for a decorative image)')
  if (requestedAlt === null) return
  const alt = requestedAlt.trim()
  const requestedCaption = window.prompt('Image caption (optional)')
  if (requestedCaption === null) return
  const image = {
    type: 'image',
    attrs: { src, alt, decorative: alt.length === 0 }
  }
  const caption = requestedCaption.trim()
  editor.value.chain().focus().insertContent(caption
    ? {
        type: 'figure',
        content: [
          image,
          { type: 'figcaption', content: [{ type: 'text', text: caption }] }
        ]
      }
    : image).run()
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
      taskItem('Add expert review notes', false),
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
  if (!editor.value || !import.meta.client) return
  editorError.value = ''
  const articleId = window.prompt('Related article ID')
  if (articleId === null) return
  const trimmed = articleId.trim()
  if (!safeReferenceID(trimmed)) {
    editorError.value = 'Related article IDs may only contain letters, numbers, hyphens, and underscores.'
    return
  }
  editor.value.chain().focus().insertContent({
    type: 'relatedReference',
    attrs: { articleId: trimmed },
    content: [
      { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: 'Related reading' }] },
      { type: 'paragraph', content: [{ type: 'text', text: 'Summarize why this article is relevant.' }] }
    ]
  }).run()
}

function insertEmbed() {
  if (!editor.value || !import.meta.client) return
  editorError.value = ''
  const provider = (window.prompt('Embed provider (youtube, vimeo, wistia)', 'youtube') || '').trim().toLowerCase()
  const url = (window.prompt('Embed URL (HTTPS from the selected provider)') || '').trim()
  if (!safeEmbedProvider(provider) || !isSafeEmbedURL(url, provider)) {
    editorError.value = 'Embeds must use an allowlisted provider URL: YouTube, Vimeo, or Wistia.'
    return
  }
  editor.value.chain().focus().insertContent({
    type: 'embedReference',
    attrs: { provider, url },
    content: [{ type: 'paragraph', content: [{ type: 'text', text: `${providerLabel(provider)} embed` }] }]
  }).run()
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
.structured-editor { overflow: hidden; border: 1px solid var(--border, #bfcac3); border-radius: 8px; background: var(--surface, #fff); }
.structured-editor:focus-within { border-color: color-mix(in srgb, var(--primary, #165a4a) 70%, white); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary, #165a4a) 14%, transparent); }
.structured-editor__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border-bottom: 1px solid var(--border, #d7ded8); background: var(--surface-subtle, #f5f7f5); }
.structured-editor__heading > div { display: grid; gap: 2px; }
.structured-editor__label { font-size: 12px; font-weight: 700; }
.structured-editor__heading small,
.structured-editor__help { color: var(--text-faint, #667169); font-size: 12px; }
.structured-editor__mode { padding: 3px 6px; border-radius: 999px; background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.structured-editor__toolbar { display: flex; flex-wrap: wrap; gap: 5px; padding: 8px; border-bottom: 1px solid var(--border, #d7ded8); background: var(--surface, #fff); }
.structured-editor__group { display: inline-flex; flex-wrap: wrap; gap: 3px; padding-right: 5px; border-right: 1px solid var(--border, #d7ded8); }
.structured-editor__group--history { margin-left: auto; padding-right: 0; border-right: 0; }
.structured-editor__select,
.structured-editor__button { min-height: 30px; border: 1px solid var(--border, #c9d4cc); border-radius: 5px; background: var(--surface, #fff); color: var(--text, #28342d); font-size: 12px; }
.structured-editor__select { padding: 0 24px 0 8px; }
.structured-editor__button { min-width: 30px; padding: 4px 7px; cursor: pointer; }
.structured-editor__button:hover:not(:disabled),
.structured-editor__button.is-active { border-color: color-mix(in srgb, var(--primary, #165a4a) 50%, var(--border, #c9d4cc)); background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); }
.structured-editor__button--danger { color: #9b2d23; }
.structured-editor__button:disabled,
.structured-editor__select:disabled { opacity: .45; cursor: not-allowed; }
.structured-editor__canvas { position: relative; background: var(--surface, #fff); }
.structured-editor__loading { display: grid; min-height: 260px; place-items: center; color: var(--text-faint, #667169); font-size: 13px; }
.structured-editor__error { margin: 0; padding: 8px 12px; border-top: 1px solid #edc6c2; background: #fff4f2; color: #9b2d23; font-size: 12px; }
.structured-editor__help { margin: 0; padding: 7px 12px; border-top: 1px solid var(--border, #d7ded8); background: var(--surface-subtle, #f5f7f5); }
.structured-editor--disabled { opacity: .75; }
:deep(.tiptap) { min-height: 260px; padding: 16px; color: var(--text, #28342d); font-size: 13px; line-height: 1.7; outline: none; }
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
  .structured-editor__group--history { margin-left: 0; }
  .structured-editor__heading { align-items: flex-start; }
  .structured-editor__mode { display: none; }
}
</style>
