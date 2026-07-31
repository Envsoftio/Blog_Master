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
import { TableKit } from '@tiptap/extension-table/kit'
import StarterKit from '@tiptap/starter-kit'
import { Plugin, PluginKey } from '@tiptap/pm/state'

const props = withDefaults(defineProps<{
  html: string
  bodyDocument?: unknown
  disabled?: boolean
  label?: string
}>(), {
  bodyDocument: undefined,
  disabled: false,
  label: 'Article body'
})

const emit = defineEmits<{
  'update:html': [value: string]
  'update:bodyDocument': [value: unknown]
}>()

const editor = shallowRef<Editor | null>(null)
const editorError = ref('')
const normalizeHeadingIDsKey = new PluginKey('normalize-article-heading-ids')

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
      Superscript,
      Subscript,
      TableKit.configure({ table: { resizable: true } })
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

function isStructuredDocument(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && (value as Record<string, unknown>).type === 'doc' && Array.isArray((value as Record<string, unknown>).content))
}

function normalizeHTML(value: string) {
  return value.trim().replace(/>\s+</g, '><')
}

function safeHeadingID(value: string) {
  return /^[A-Za-z][A-Za-z0-9_-]{0,127}$/.test(value)
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
</script>

<style scoped>
.structured-editor { overflow: hidden; border: 1px solid var(--border, #bfcac3); border-radius: 8px; background: var(--surface, #fff); }
.structured-editor:focus-within { border-color: color-mix(in srgb, var(--primary, #165a4a) 70%, white); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary, #165a4a) 14%, transparent); }
.structured-editor__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border-bottom: 1px solid var(--border, #d7ded8); background: var(--surface-subtle, #f5f7f5); }
.structured-editor__heading > div { display: grid; gap: 2px; }
.structured-editor__label { font-size: 12px; font-weight: 700; }
.structured-editor__heading small,
.structured-editor__help { color: var(--text-faint, #667169); font-size: 9px; }
.structured-editor__mode { padding: 3px 6px; border-radius: 999px; background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); font-size: 8px; font-weight: 700; text-transform: uppercase; }
.structured-editor__toolbar { display: flex; flex-wrap: wrap; gap: 5px; padding: 8px; border-bottom: 1px solid var(--border, #d7ded8); background: var(--surface, #fff); }
.structured-editor__group { display: inline-flex; flex-wrap: wrap; gap: 3px; padding-right: 5px; border-right: 1px solid var(--border, #d7ded8); }
.structured-editor__group--history { margin-left: auto; padding-right: 0; border-right: 0; }
.structured-editor__select,
.structured-editor__button { min-height: 30px; border: 1px solid var(--border, #c9d4cc); border-radius: 5px; background: var(--surface, #fff); color: var(--text, #28342d); font-size: 10px; }
.structured-editor__select { padding: 0 24px 0 8px; }
.structured-editor__button { min-width: 30px; padding: 4px 7px; cursor: pointer; }
.structured-editor__button:hover:not(:disabled),
.structured-editor__button.is-active { border-color: color-mix(in srgb, var(--primary, #165a4a) 50%, var(--border, #c9d4cc)); background: var(--primary-soft, #e6f2ec); color: var(--primary, #165a4a); }
.structured-editor__button--danger { color: #9b2d23; }
.structured-editor__button:disabled,
.structured-editor__select:disabled { opacity: .45; cursor: not-allowed; }
.structured-editor__canvas { position: relative; background: var(--surface, #fff); }
.structured-editor__loading { display: grid; min-height: 260px; place-items: center; color: var(--text-faint, #667169); font-size: 11px; }
.structured-editor__error { margin: 0; padding: 8px 12px; border-top: 1px solid #edc6c2; background: #fff4f2; color: #9b2d23; font-size: 10px; }
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
:deep(.tiptap figure) { margin: 0 0 12px; }
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
:deep(.tiptap img) { max-width: 100%; height: auto; border-radius: 6px; }
:deep(.tiptap figure) { padding: 8px; border: 1px solid var(--border, #d7ded8); border-radius: 6px; }
:deep(.tiptap figcaption) { margin-top: 6px; color: var(--text-faint, #667169); font-size: 10px; }
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
