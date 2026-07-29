<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <NuxtLink
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            :to="`/projects/${projectID}/articles`"
            title="Back to articles"
            aria-label="Back to articles"
          >
            <ArrowLeft class="h-4 w-4" />
          </NuxtLink>
          <div class="min-w-0">
            <p class="truncate text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ project?.name || 'Project' }}</p>
            <h1 class="mt-1 truncate text-2xl font-semibold tracking-normal">{{ article?.title || 'Article' }}</h1>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Refresh"
            aria-label="Refresh"
            :disabled="pending"
            @click="refresh"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
          </button>
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#fff4df] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Log out"
            aria-label="Log out"
            @click="logout"
          >
            <LogOut class="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>

    <div class="mx-auto grid max-w-7xl grid-cols-1 gap-6 px-6 py-6 lg:grid-cols-[220px_1fr]">
      <ProjectNav :project-id="projectID" :project="project" active="article-detail" />

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading article
        </div>

        <div v-else-if="article" class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
          <div class="space-y-5">
            <article class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ labelize(article.articleType) }} / {{ article.locale }}</p>
                  <h2 class="mt-1 truncate text-xl font-semibold tracking-normal">{{ article.title }}</h2>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ article.slug }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="editorialClass(article.editorialState)">{{ labelize(article.editorialState) }}</span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="publicationClass(article.publicationState)">{{ labelize(article.publicationState) }}</span>
                </div>
              </div>

              <dl class="mt-5 grid gap-3 text-sm md:grid-cols-3">
                <div class="flex items-center gap-2">
                  <Hash class="h-4 w-4 text-[#3162a3]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Article ID</dt>
                    <dd class="truncate font-mono">{{ article.id }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <CalendarClock class="h-4 w-4 text-[#8a5b00]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Scheduled</dt>
                    <dd class="truncate">{{ formatDate(article.scheduledForUtc) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <UploadCloud class="h-4 w-4 text-[#165a4a]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Published</dt>
                    <dd class="truncate">{{ formatDate(article.publishedAt) }}</dd>
                  </div>
                </div>
              </dl>

              <div v-if="article.canonicalUrl" class="mt-4 truncate rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">
                {{ article.canonicalUrl }}
              </div>
            </article>

            <article class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Latest immutable revision</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">{{ article.latestRevision ? `Revision #${article.latestRevision.revisionNumber}` : 'No revision' }}</h2>
                </div>
                <span v-if="article.latestRevision" class="rounded-full px-2.5 py-1 text-xs font-medium" :class="editorialClass(article.latestRevision.editorialState)">
                  {{ labelize(article.latestRevision.editorialState) }}
                </span>
              </div>

              <dl v-if="article.latestRevision" class="mt-5 grid gap-3 text-sm md:grid-cols-2">
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Revision ID</dt>
                  <dd class="truncate font-mono">{{ article.latestRevision.id }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Content hash</dt>
                  <dd class="truncate font-mono">{{ article.latestRevision.contentHash }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Created</dt>
                  <dd class="truncate">{{ formatDate(article.latestRevision.createdAt) }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Locale</dt>
                  <dd class="truncate">{{ article.latestRevision.locale }}</dd>
                </div>
              </dl>

              <p v-if="article.latestRevision?.deck" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ article.latestRevision.deck }}</p>
              <p v-if="article.latestRevision?.excerpt" class="mt-2 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ article.latestRevision.excerpt }}</p>
              <p v-if="article.latestRevision?.shortAnswer" class="mt-2 rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">{{ article.latestRevision.shortAnswer }}</p>

              <div class="mt-5 flex flex-wrap gap-2">
                <button
                  v-if="article.editorialState === 'draft' || article.editorialState === 'changes_requested'"
                  class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="actionPending === 'submit'"
                  @click="submitRevision"
                >
                  <Send class="h-4 w-4" />
                  Submit
                </button>
                <button
                  v-if="article.editorialState === 'in_review'"
                  class="inline-flex items-center gap-2 rounded-md border border-[#d6bd7a] px-3 py-2 text-sm font-medium text-[#7a4f00] hover:bg-[#fff7e4] disabled:opacity-60 dark:border-[#6b572e] dark:text-[#ffd98a] dark:hover:bg-[#2b2415]"
                  type="button"
                  :disabled="actionPending === 'request-changes'"
                  @click="requestChanges"
                >
                  <RotateCcw class="h-4 w-4" />
                  Request changes
                </button>
                <button
                  v-if="article.editorialState !== 'approved'"
                  class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="actionPending === 'approve'"
                  @click="approveRevision"
                >
                  <CheckCircle2 class="h-4 w-4" />
                  Approve
                </button>
              </div>
            </article>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Review thread</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Comments</h2>
                </div>
                <span class="rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">{{ openCommentCount }} open</span>
              </div>

              <form class="mt-5 space-y-3" @submit.prevent="createComment">
                <div class="grid gap-3 sm:grid-cols-2">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">Revision ID</span>
                    <input v-model.trim="commentForm.revisionId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" />
                  </label>
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">Block ID</span>
                    <input v-model.trim="commentForm.blockId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" />
                  </label>
                </div>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Comment</span>
                  <textarea v-model.trim="commentForm.body" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
                </label>
                <button
                  class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                  type="submit"
                  :disabled="creatingComment || !commentForm.body.trim()"
                >
                  <LoaderCircle v-if="creatingComment" class="h-4 w-4 animate-spin" />
                  <MessageSquarePlus v-else class="h-4 w-4" />
                  Add comment
                </button>
              </form>

              <div v-if="comments.length === 0" class="mt-5 rounded-lg border border-dashed border-[#bfcac3] p-6 text-center dark:border-[#4b5650]">
                <h3 class="text-lg font-semibold">No comments yet</h3>
              </div>

              <article v-for="comment in comments" :key="comment.id" class="mt-4 rounded-lg border border-[#cfd8d1] p-4 dark:border-[#3f4843]">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ comment.body }}</p>
                    <p class="mt-2 truncate font-mono text-xs text-[#667169] dark:text-[#aeb8b0]">{{ comment.revisionId || 'article' }} {{ comment.blockId || '' }}</p>
                  </div>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="commentStatusClass(comment.status)">{{ comment.status }}</span>
                </div>
                <dl class="mt-4 grid gap-3 text-sm sm:grid-cols-3">
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Created</dt>
                    <dd>{{ formatDate(comment.createdAt) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Created by</dt>
                    <dd class="truncate font-mono">{{ comment.createdBy }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Resolved</dt>
                    <dd>{{ formatDate(comment.resolvedAt) }}</dd>
                  </div>
                </dl>
                <div class="mt-4 flex flex-wrap gap-2">
                  <button
                    v-if="comment.status !== 'resolved'"
                    class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    :disabled="commentPending[comment.id] === 'resolve'"
                    @click="setCommentStatus(comment, 'resolve')"
                  >
                    <CheckCircle2 class="h-4 w-4" />
                    Resolve
                  </button>
                  <button
                    v-else
                    class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    :disabled="commentPending[comment.id] === 'reopen'"
                    @click="setCommentStatus(comment, 'reopen')"
                  >
                    <RotateCcw class="h-4 w-4" />
                    Reopen
                  </button>
                </div>
              </article>

              <button
                v-if="nextCommentCursor"
                class="mt-5 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
                type="button"
                :disabled="loadingMoreComments"
                @click="loadMoreComments"
              >
                <LoaderCircle v-if="loadingMoreComments" class="h-4 w-4 animate-spin" />
                <RefreshCw v-else class="h-4 w-4" />
                Load more comments
              </button>
            </section>
          </div>

          <div class="space-y-5">
            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createRevision">
              <div class="flex items-start gap-3">
                <FilePenLine class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Draft</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">New revision</h2>
                </div>
              </div>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Title</span>
                <input v-model.trim="revisionForm.title" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Primary category</span>
                <select v-model="revisionForm.primaryCategoryId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option value="">Keep current category</option>
                  <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Deck</span>
                <textarea v-model.trim="revisionForm.deck" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Excerpt</span>
                <textarea v-model.trim="revisionForm.excerpt" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Short answer</span>
                <textarea v-model.trim="revisionForm.shortAnswer" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">HTML</span>
                <textarea v-model.trim="revisionForm.html" class="min-h-40 w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>

              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="creatingRevision || !canCreateRevision"
              >
                <LoaderCircle v-if="creatingRevision" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create revision
              </button>
            </form>

            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="publishArticle">
              <div class="flex items-start gap-3">
                <UploadCloud class="mt-1 h-4 w-4 text-[#165a4a]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Publication</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Publish pointer</h2>
                </div>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="publicationForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Canonical URL</span>
                <input v-model.trim="publicationForm.canonicalUrl" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="url" />
              </label>
              <div class="grid gap-2">
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                  type="submit"
                  :disabled="actionPending === 'publish' || article.editorialState !== 'approved'"
                >
                  <UploadCloud class="h-4 w-4" />
                  Publish
                </button>
                <button
                  v-if="article.publicationState !== 'unpublished'"
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-4 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                  type="button"
                  :disabled="actionPending === 'unpublish'"
                  @click="unpublishArticle"
                >
                  <XCircle class="h-4 w-4" />
                  Unpublish
                </button>
              </div>
            </form>

            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="scheduleArticle">
              <div class="flex items-start gap-3">
                <CalendarClock class="mt-1 h-4 w-4 text-[#8a5b00]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Schedule</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Timed publish</h2>
                </div>
              </div>
              <input
                v-model="scheduleDraft"
                class="h-10 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
                type="datetime-local"
                required
              />
              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="actionPending === 'schedule' || article.editorialState !== 'approved' || !scheduleDraft"
              >
                <CalendarClock class="h-4 w-4" />
                Schedule
              </button>
            </form>

            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="rollbackArticle">
              <div class="flex items-start gap-3">
                <History class="mt-1 h-4 w-4 text-[#6b5797]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Restore</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Rollback</h2>
                </div>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Approved revision ID</span>
                <input v-model.trim="rollbackForm.revisionId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="actionPending === 'rollback' || !rollbackForm.revisionId"
              >
                <History class="h-4 w-4" />
                Rollback
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  ArrowLeft,
  CalendarClock,
  CheckCircle2,
  FilePenLine,
  Hash,
  History,
  LoaderCircle,
  LogOut,
  MessageSquarePlus,
  Plus,
  RefreshCw,
  RotateCcw,
  Send,
  UploadCloud,
  XCircle
} from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
  meta?: {
    nextCursor?: string
    limit: number
  }
}

type AdminProject = {
  id: string
  slug: string
  name: string
  status: string
  role: string
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
}

type AdminRevision = {
  id: string
  projectId: string
  articleId: string
  revisionNumber: number
  title: string
  deck?: string
  excerpt?: string
  shortAnswer?: string
  locale: string
  editorialState: string
  contentHash: string
  createdAt: string
}

type AdminArticle = {
  id: string
  projectId: string
  articleType: string
  slug: string
  locale: string
  title: string
  editorialState: string
  publicationState: string
  scheduledForUtc?: string
  publishedAt?: string
  canonicalUrl?: string
  latestRevision?: AdminRevision
  createdAt: string
}

type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  parentId?: string
  indexable: boolean
}

type ReviewComment = {
  id: string
  projectId: string
  articleId: string
  revisionId?: string
  blockId?: string
  body: string
  status: string
  createdBy: string
  createdAt: string
  resolvedBy?: string
  resolvedAt?: string
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})
const articleID = computed(() => {
  const value = route.params.articleId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const project = ref<AdminProject | null>(null)
const article = ref<AdminArticle | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const comments = ref<ReviewComment[]>([])
const pending = ref(true)
const creatingRevision = ref(false)
const creatingComment = ref(false)
const loadingMoreComments = ref(false)
const nextCommentCursor = ref('')
const actionPending = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const commentPending = reactive<Record<string, string>>({})

const revisionForm = reactive({
  title: '',
  primaryCategoryId: '',
  deck: '',
  excerpt: '',
  shortAnswer: '',
  html: ''
})

const publicationForm = reactive({
  slug: '',
  canonicalUrl: ''
})

const rollbackForm = reactive({
  revisionId: ''
})

const commentForm = reactive({
  revisionId: '',
  blockId: '',
  body: ''
})

const scheduleDraft = ref('')
const canCreateRevision = computed(() => Boolean(revisionForm.title.trim()))
const openCommentCount = computed(() => comments.value.filter(comment => comment.status !== 'resolved').length)

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, categoryResponse, articleResponse, commentResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, {
        credentials: 'include',
        query: { limit: 100 }
      }),
      $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<ReviewComment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/comments`, {
        credentials: 'include',
        query: { limit: 50 }
      })
    ])
    project.value = projectResponse.data
    categories.value = sortCategories(categoryResponse.data)
    setArticle(articleResponse.data)
    comments.value = commentResponse.data
    nextCommentCursor.value = commentResponse.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load article. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function createRevision() {
  if (!article.value) return
  creatingRevision.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/articles/${article.value.id}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        title: revisionForm.title,
        primaryCategoryId: revisionForm.primaryCategoryId,
        deck: revisionForm.deck,
        excerpt: revisionForm.excerpt,
        shortAnswer: revisionForm.shortAnswer,
        html: revisionForm.html || `<p>${escapeHTML(revisionForm.title)}</p>`
      }
    })
    revisionForm.title = ''
    revisionForm.primaryCategoryId = ''
    revisionForm.deck = ''
    revisionForm.excerpt = ''
    revisionForm.shortAnswer = ''
    revisionForm.html = ''
    successMessage.value = `Revision #${response.data.revisionNumber} created.`
    await fetchArticle()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create revision.')
  } finally {
    creatingRevision.value = false
  }
}

async function submitRevision() {
  await mutateArticle('submit', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/revisions/${latestRevisionID()}/submit`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Revision submitted.'
  })
}

async function requestChanges() {
  await mutateArticle('request-changes', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/revisions/${latestRevisionID()}/request-changes`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Changes requested.'
  })
}

async function approveRevision() {
  await mutateArticle('approve', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/revisions/${latestRevisionID()}/approve`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Revision approved.'
  })
}

async function publishArticle() {
  await mutateArticle('publish', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/publish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: publicationBody(latestRevisionID())
    })
    successMessage.value = 'Article published.'
  })
}

async function scheduleArticle() {
  await mutateArticle('schedule', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/schedule`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        ...publicationBody(latestRevisionID()),
        scheduledForUtc: new Date(scheduleDraft.value).toISOString()
      }
    })
    successMessage.value = 'Article scheduled.'
  })
}

async function unpublishArticle() {
  if (!window.confirm('Unpublish this article?')) return
  await mutateArticle('unpublish', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/unpublish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Article unpublished.'
  })
}

async function rollbackArticle() {
  if (!window.confirm('Rollback to this approved revision?')) return
  await mutateArticle('rollback', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/rollback`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        revisionId: rollbackForm.revisionId,
        locale: article.value?.locale || project.value?.defaultLocale || 'en'
      }
    })
    rollbackForm.revisionId = ''
    successMessage.value = 'Article rolled back.'
  })
}

async function mutateArticle(action: string, operation: (csrfToken: string) => Promise<void>) {
  if (!article.value?.latestRevision) {
    errorMessage.value = 'This article has no revision.'
    return
  }
  actionPending.value = action
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    await operation(csrfToken)
    await fetchArticle()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, `Could not ${labelize(action)} article.`)
  } finally {
    actionPending.value = ''
  }
}

async function createComment() {
  creatingComment.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ReviewComment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/comments`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        revisionId: commentForm.revisionId,
        blockId: commentForm.blockId,
        body: commentForm.body
      }
    })
    comments.value = [response.data, ...comments.value]
    commentForm.blockId = ''
    commentForm.body = ''
    successMessage.value = 'Comment added.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not add comment.')
  } finally {
    creatingComment.value = false
  }
}

async function setCommentStatus(comment: ReviewComment, transition: 'resolve' | 'reopen') {
  commentPending[comment.id] = transition
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ReviewComment>>(`/api/v1/projects/${projectID.value}/comments/${comment.id}/${transition}`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    upsertComment(response.data)
    successMessage.value = transition === 'resolve' ? 'Comment resolved.' : 'Comment reopened.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, transition === 'resolve' ? 'Could not resolve comment.' : 'Could not reopen comment.')
  } finally {
    delete commentPending[comment.id]
  }
}

async function loadMoreComments() {
  if (!nextCommentCursor.value || loadingMoreComments.value) return
  loadingMoreComments.value = true
  clearMessages()
  try {
    const response = await $fetch<APIListEnvelope<ReviewComment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/comments`, {
      credentials: 'include',
      query: {
        cursor: nextCommentCursor.value,
        limit: 50
      }
    })
    const merged = new Map(comments.value.map(comment => [comment.id, comment]))
    for (const comment of response.data) merged.set(comment.id, comment)
    comments.value = [...merged.values()]
    nextCommentCursor.value = response.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more comments.')
  } finally {
    loadingMoreComments.value = false
  }
}

async function fetchArticle() {
  const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}`, {
    credentials: 'include'
  })
  setArticle(response.data)
}

function setArticle(value: AdminArticle) {
  article.value = value
  publicationForm.slug = value.slug
  publicationForm.canonicalUrl = value.canonicalUrl || ''
  revisionForm.title = revisionForm.title || value.title
  commentForm.revisionId = value.latestRevision?.id || ''
  if (!scheduleDraft.value) {
    const scheduledAt = value.scheduledForUtc ? parseBackendUTC(value.scheduledForUtc) : new Date(Date.now() + 15 * 60 * 1000)
    scheduleDraft.value = toLocalInputValue(scheduledAt)
  }
}

function publicationBody(revisionId: string) {
  return {
    revisionId,
    slug: publicationForm.slug,
    locale: article.value?.locale || project.value?.defaultLocale || 'en',
    canonicalUrl: publicationForm.canonicalUrl || undefined
  }
}

function latestRevisionID() {
  return article.value?.latestRevision?.id || ''
}

function upsertComment(comment: ReviewComment) {
  const index = comments.value.findIndex(item => item.id === comment.id)
  if (index >= 0) {
    comments.value.splice(index, 1, comment)
  } else {
    comments.value = [comment, ...comments.value]
  }
}

async function logout() {
  try {
    const csrfToken = await getCSRFToken()
    await $fetch('/api/v1/auth/logout', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
  } finally {
    await navigateTo('/')
  }
}

async function getCSRFToken() {
  const response = await $fetch<APIEnvelope<{ csrfToken: string }>>('/api/v1/auth/csrf', {
    credentials: 'include'
  })
  return response.data.csrfToken
}

function sortCategories(values: TaxonomyTerm[]) {
  return [...values].sort((left, right) => left.name.localeCompare(right.name))
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
}

function toLocalInputValue(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = parseBackendUTC(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}

function editorialClass(state: string) {
  switch (state) {
    case 'approved':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'in_review':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'changes_requested':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function publicationClass(state: string) {
  switch (state) {
    case 'published':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'scheduled':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function commentStatusClass(status: string) {
  switch (status) {
    case 'resolved':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'reopened':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function labelize(value: string) {
  return value.replaceAll('_', ' ').replaceAll('-', ' ')
}

function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || fallback
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}
</script>
