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
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ labelize(article.articleType) }}</p>
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
              </dl>

              <p v-if="article.latestRevision?.deck" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ article.latestRevision.deck }}</p>
              <p v-if="article.latestRevision?.excerpt" class="mt-2 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ article.latestRevision.excerpt }}</p>
              <p v-if="article.latestRevision?.shortAnswer" class="mt-2 rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">{{ article.latestRevision.shortAnswer }}</p>

              <div class="mt-5 flex flex-wrap gap-2">
                <button
                  v-if="canWriteArticles && (article.editorialState === 'draft' || article.editorialState === 'changes_requested')"
                  class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="actionPending === 'submit'"
                  @click="submitRevision"
                >
                  <Send class="h-4 w-4" />
                  Submit
                </button>
                <button
                  v-if="canReviewArticles && article.editorialState === 'in_review'"
                  class="inline-flex items-center gap-2 rounded-md border border-[#d6bd7a] px-3 py-2 text-sm font-medium text-[#7a4f00] hover:bg-[#fff7e4] disabled:opacity-60 dark:border-[#6b572e] dark:text-[#ffd98a] dark:hover:bg-[#2b2415]"
                  type="button"
                  :disabled="actionPending === 'request-changes'"
                  @click="requestChanges"
                >
                  <RotateCcw class="h-4 w-4" />
                  Request changes
                </button>
                <button
                  v-if="canReviewArticles && article.editorialState === 'in_review'"
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

            <ArticleTrustPanel
              v-if="article.latestRevision"
              :project-id="projectID"
              :article-id="articleID"
              :revision-id="article.latestRevision.id"
              :role="project?.role || ''"
            />

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Immutable history</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Compare revisions</h2>
                </div>
                <span class="rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">
                  {{ revisions.length }} loaded
                </span>
              </div>

              <ol class="mt-5 grid gap-3 sm:grid-cols-2" aria-label="Article revision history">
                <li
                  v-for="revision in revisions"
                  :key="revision.id"
                  class="rounded-lg border border-[#d7ded8] p-4 dark:border-[#3f4843]"
                >
                  <div class="flex flex-wrap items-start justify-between gap-2">
                    <div>
                      <p class="font-medium">Revision #{{ revision.revisionNumber }}</p>
                      <p class="mt-1 text-xs text-[#667169] dark:text-[#aeb8b0]">{{ formatDate(revision.createdAt) }}</p>
                    </div>
                    <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="editorialClass(revision.editorialState)">
                      {{ labelize(revision.editorialState) }}
                    </span>
                  </div>
                  <p class="mt-3 line-clamp-2 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ revision.title }}</p>
                  <p v-if="revision.published" class="mt-2 text-xs font-medium text-[#165a4a] dark:text-[#aee4d0]">Published</p>
                  <div class="mt-3 flex flex-wrap gap-2">
                    <button
                      class="rounded-md border border-[#c9d4cc] px-2.5 py-1.5 text-xs font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                      :class="{ 'bg-[#e6f2ec] text-[#165a4a] dark:bg-[#17352c] dark:text-[#aee4d0]': comparisonForm.beforeRevisionId === revision.id }"
                      type="button"
                      :aria-label="`Use revision ${revision.revisionNumber} as comparison A`"
                      :aria-pressed="comparisonForm.beforeRevisionId === revision.id"
                      :disabled="comparisonPending"
                      @click="selectRevisionForComparison('before', revision.id)"
                    >
                      Compare from
                    </button>
                    <button
                      class="rounded-md border border-[#c9d4cc] px-2.5 py-1.5 text-xs font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                      :class="{ 'bg-[#e6f2ec] text-[#165a4a] dark:bg-[#17352c] dark:text-[#aee4d0]': comparisonForm.afterRevisionId === revision.id }"
                      type="button"
                      :aria-label="`Use revision ${revision.revisionNumber} as comparison B`"
                      :aria-pressed="comparisonForm.afterRevisionId === revision.id"
                      :disabled="comparisonPending"
                      @click="selectRevisionForComparison('after', revision.id)"
                    >
                      Compare to
                    </button>
                  </div>
                </li>
              </ol>

              <button
                v-if="nextRevisionCursor"
                class="mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="button"
                :disabled="loadingMoreRevisions"
                @click="loadMoreRevisions"
              >
                <LoaderCircle v-if="loadingMoreRevisions" class="h-4 w-4 animate-spin" />
                <RefreshCw v-else class="h-4 w-4" />
                Load older revisions
              </button>

              <form class="mt-5 grid items-end gap-3 rounded-lg bg-[#f5f7f5] p-4 dark:bg-[#171b18] md:grid-cols-[1fr_1fr_auto]" @submit.prevent="compareRevisions">
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Revision A</span>
                  <select v-model="comparisonForm.beforeRevisionId" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#202522]" :disabled="comparisonPending">
                    <option v-for="revision in revisions" :key="revision.id" :value="revision.id">
                      #{{ revision.revisionNumber }} · {{ revision.title }}
                    </option>
                  </select>
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Revision B</span>
                  <select v-model="comparisonForm.afterRevisionId" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#202522]" :disabled="comparisonPending">
                    <option v-for="revision in revisions" :key="revision.id" :value="revision.id">
                      #{{ revision.revisionNumber }} · {{ revision.title }}
                    </option>
                  </select>
                </label>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                  type="submit"
                  :disabled="comparisonPending || !canCompareRevisions"
                >
                  <LoaderCircle v-if="comparisonPending" class="h-4 w-4 animate-spin" />
                  <GitCompareArrows v-else class="h-4 w-4" />
                  Compare
                </button>
              </form>

              <p
                v-if="comparisonBefore && comparisonAfter"
                class="mt-5 rounded-md border border-[#c9d4cc] bg-[#f2f5f3] px-4 py-3 text-sm text-[#4f5b54] dark:border-[#414a45] dark:bg-[#171b18] dark:text-[#c5cec8]"
                role="status"
                aria-live="polite"
              >
                {{ comparisonSummary }}
              </p>

              <div v-if="comparisonBefore && comparisonAfter" class="mt-4 space-y-4">
                <div class="grid gap-3 text-sm sm:grid-cols-2">
                  <div class="rounded-md border border-[#d7ded8] p-3 dark:border-[#3f4843]">
                    <p class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Earlier</p>
                    <p class="mt-1 font-medium">Revision #{{ comparisonBefore.revisionNumber }}</p>
                    <p class="mt-1 text-xs">{{ formatDate(comparisonBefore.createdAt) }}</p>
                  </div>
                  <div class="rounded-md border border-[#d7ded8] p-3 dark:border-[#3f4843]">
                    <p class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Later</p>
                    <p class="mt-1 font-medium">Revision #{{ comparisonAfter.revisionNumber }}</p>
                    <p class="mt-1 text-xs">{{ formatDate(comparisonAfter.createdAt) }}</p>
                  </div>
                </div>

                <article
                  v-for="field in comparisonFields"
                  :key="field.key"
                  class="rounded-lg border p-4"
                  :class="field.changed
                    ? 'border-[#d6bd7a] bg-[#fffaf0] dark:border-[#6b572e] dark:bg-[#2b2415]'
                    : 'border-[#d7ded8] dark:border-[#3f4843]'"
                >
                  <div class="flex items-center justify-between gap-3">
                    <h3 class="text-sm font-semibold">{{ field.label }}</h3>
                    <span v-if="field.changed" class="rounded-full bg-[#fff0ce] px-2 py-1 text-xs font-medium text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]">Changed</span>
                    <span v-else class="text-xs text-[#667169] dark:text-[#aeb8b0]">Unchanged</span>
                  </div>
                  <div class="mt-3 grid gap-3 sm:grid-cols-2">
                    <div>
                      <p class="mb-1 text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Earlier</p>
                      <pre
                        class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md bg-white p-3 text-sm dark:bg-[#171b18]"
                        :class="{ 'font-mono text-xs': field.monospace }"
                        :aria-label="`${field.label} in revision ${comparisonBefore.revisionNumber}`"
                        tabindex="0"
                      >{{ field.before || '—' }}</pre>
                    </div>
                    <div>
                      <p class="mb-1 text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Later</p>
                      <pre
                        class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md bg-white p-3 text-sm dark:bg-[#171b18]"
                        :class="{ 'font-mono text-xs': field.monospace }"
                        :aria-label="`${field.label} in revision ${comparisonAfter.revisionNumber}`"
                        tabindex="0"
                      >{{ field.after || '—' }}</pre>
                    </div>
                  </div>
                  <details v-if="field.changed && field.diffLines?.length" class="mt-3 rounded-md border border-[#d6bd7a] bg-white dark:border-[#6b572e] dark:bg-[#171b18]">
                    <summary class="cursor-pointer px-3 py-2 text-sm font-medium">Show inline changes</summary>
                    <ol
                      class="max-h-96 overflow-auto border-t border-[#ead9ad] font-mono text-xs dark:border-[#574927]"
                      :aria-label="`${field.label} inline changes from revision ${comparisonBefore.revisionNumber} to revision ${comparisonAfter.revisionNumber}`"
                      tabindex="0"
                    >
                      <li
                        v-for="(line, index) in field.diffLines"
                        :key="`${field.key}-${index}`"
                        class="grid grid-cols-[2rem_1fr] gap-2 px-3 py-1"
                        :class="diffLineClass(line.kind)"
                      >
                        <span aria-hidden="true">{{ diffLineMarker(line.kind) }}</span>
                        <span class="sr-only">{{ diffLineLabel(line.kind) }}: </span>
                        <code class="whitespace-pre-wrap break-words">{{ line.text || ' ' }}</code>
                      </li>
                    </ol>
                  </details>
                </article>
              </div>
            </section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Review ownership</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Assignments</h2>
                </div>
                <span class="rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">{{ openAssignmentCount }} open</span>
              </div>

              <form v-if="canManageAssignments" class="mt-5 grid gap-3 lg:grid-cols-[minmax(0,1fr)_150px_190px_auto]" @submit.prevent="createAssignment">
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Assignee</span>
                  <select v-model="assignmentForm.assignedTo" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required>
                    <option value="">Select a member</option>
                    <option v-for="member in assignmentEligibleMembers" :key="member.userId" :value="member.userId">
                      {{ member.email }} · {{ labelize(member.role) }}
                    </option>
                  </select>
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Role</span>
                  <select v-model="assignmentForm.assignmentType" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
                    <option value="reviewer">Reviewer</option>
                    <option value="editor">Editor</option>
                    <option value="sme">SME</option>
                  </select>
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Due</span>
                  <input v-model="assignmentForm.dueAt" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" type="datetime-local" />
                </label>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 self-end rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                  type="submit"
                  :disabled="creatingAssignment || !canCreateAssignment"
                >
                  <LoaderCircle v-if="creatingAssignment" class="h-4 w-4 animate-spin" />
                  <UserCheck v-else class="h-4 w-4" />
                  Assign
                </button>
              </form>

              <div v-if="assignments.length === 0" class="mt-5 rounded-lg border border-dashed border-[#bfcac3] p-6 text-center dark:border-[#4b5650]">
                <h3 class="text-lg font-semibold">No assignments yet</h3>
              </div>

              <article v-for="assignment in assignments" :key="assignment.id" class="mt-4 rounded-lg border border-[#cfd8d1] p-4 dark:border-[#3f4843]">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium">{{ assignment.assigneeEmail || assignment.assignedTo }}</p>
                    <p class="mt-1 truncate font-mono text-xs text-[#667169] dark:text-[#aeb8b0]">{{ assignment.revisionId || 'article' }}</p>
                  </div>
                  <div class="flex flex-wrap justify-end gap-2">
                    <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="assignmentTypeClass(assignment.assignmentType)">{{ labelize(assignment.assignmentType) }}</span>
                    <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="assignmentStatusClass(assignment.status)">{{ labelize(assignment.status) }}</span>
                  </div>
                </div>
                <dl class="mt-4 grid gap-3 text-sm sm:grid-cols-3">
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Due</dt>
                    <dd>{{ formatDate(assignment.dueAt) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Created</dt>
                    <dd>{{ formatDate(assignment.createdAt) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">{{ assignment.closedAt ? 'Closed' : 'Created by' }}</dt>
                    <dd v-if="assignment.closedAt">{{ formatDate(assignment.closedAt) }}</dd>
                    <dd v-else class="truncate font-mono">{{ assignment.createdBy }}</dd>
                  </div>
                </dl>
                <div v-if="assignment.status === 'open' && (canCompleteAssignment(assignment) || canManageAssignments)" class="mt-4 flex flex-wrap justify-end gap-2 border-t border-[#dce4de] pt-4 dark:border-[#39413d]">
                  <button
                    v-if="canCompleteAssignment(assignment)"
                    class="inline-flex h-9 items-center gap-2 rounded-md border border-[#9bc8b6] px-3 text-sm font-medium text-[#165a4a] hover:bg-[#e0f3e9] disabled:opacity-60 dark:border-[#376557] dark:text-[#aee4d0] dark:hover:bg-[#12382f]"
                    type="button"
                    :disabled="Boolean(assignmentPending[assignment.id])"
                    @click="setAssignmentStatus(assignment, 'complete')"
                  >
                    <LoaderCircle v-if="assignmentPending[assignment.id] === 'complete'" class="h-4 w-4 animate-spin" />
                    <CheckCircle2 v-else class="h-4 w-4" />
                    Complete
                  </button>
                  <button
                    v-if="canManageAssignments"
                    class="inline-flex h-9 items-center gap-2 rounded-md border border-[#d8b078] px-3 text-sm font-medium text-[#7a4f00] hover:bg-[#fff0ce] disabled:opacity-60 dark:border-[#6e5726] dark:text-[#ffd98a] dark:hover:bg-[#3a2d12]"
                    type="button"
                    :disabled="Boolean(assignmentPending[assignment.id])"
                    @click="setAssignmentStatus(assignment, 'cancel')"
                  >
                    <LoaderCircle v-if="assignmentPending[assignment.id] === 'cancel'" class="h-4 w-4 animate-spin" />
                    <XCircle v-else class="h-4 w-4" />
                    Cancel
                  </button>
                </div>
              </article>

              <button
                v-if="nextAssignmentCursor"
                class="mt-5 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
                type="button"
                :disabled="loadingMoreAssignments"
                @click="loadMoreAssignments"
              >
                <LoaderCircle v-if="loadingMoreAssignments" class="h-4 w-4 animate-spin" />
                <RefreshCw v-else class="h-4 w-4" />
                Load more assignments
              </button>
            </section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Review thread</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Comments</h2>
                </div>
                <span class="rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">{{ openCommentCount }} open</span>
              </div>

              <form v-if="canComment" class="mt-5 space-y-3" @submit.prevent="createComment">
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
                <div v-if="canComment" class="mt-4 flex flex-wrap gap-2">
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
            <form v-if="canWriteArticles" class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createRevision">
              <div class="flex items-start gap-3">
                <FilePenLine class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div class="min-w-0 flex-1">
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Draft</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">New revision</h2>
                  <p v-if="draftStatusText" class="mt-1 text-xs" :class="draftStatusClass" aria-live="polite">
                    {{ draftStatusText }}
                  </p>
                </div>
              </div>

              <div
                v-if="staleDraft"
                class="rounded-md border border-[#e1bd70] bg-[#fff8e7] p-3 text-sm text-[#6b4905] dark:border-[#665223] dark:bg-[#2b2415] dark:text-[#f5d992]"
              >
                <p class="font-medium">{{ staleDraft.reason === 'version-conflict' ? 'Another tab saved a different draft.' : 'A working draft was saved against an older revision.' }}</p>
                <p class="mt-1 text-xs">
                  {{ staleDraft.reason === 'version-conflict'
                    ? `This tab's browser backup from ${formatDate(staleDraft.snapshot.savedAt)} is still available for manual reconciliation.`
                    : `The article changed after this ${staleDraft.source === 'server' ? 'server' : 'browser'} draft was saved ${formatDate(staleDraft.snapshot.savedAt)}. Restore it for manual reconciliation, or discard it.` }}
                </p>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button class="rounded-md border border-current px-3 py-1.5 text-xs font-medium" type="button" @click="restoreStaleDraft">Restore for reconciliation</button>
                  <button class="rounded-md px-3 py-1.5 text-xs font-medium underline" type="button" @click="discardLocalDraft">Discard</button>
                </div>
              </div>

              <div
                v-else-if="serverDraftSaveState === 'conflict'"
                class="rounded-md border border-[#e1bd70] bg-[#fff8e7] p-3 text-sm text-[#6b4905] dark:border-[#665223] dark:bg-[#2b2415] dark:text-[#f5d992]"
              >
                <p class="font-medium">Autosave paused because another tab saved newer work.</p>
                <p class="mt-1 text-xs">Reload the server draft to compare it with this tab's browser backup before continuing.</p>
                <button class="mt-3 rounded-md border border-current px-3 py-1.5 text-xs font-medium disabled:opacity-60" type="button" :disabled="reloadingServerDraft" @click="reloadServerDraft">
                  {{ reloadingServerDraft ? 'Reloading…' : 'Reload server draft' }}
                </button>
              </div>

              <p
                v-else-if="draftSaveState === 'restored'"
                class="rounded-md border border-[#b9d5c8] bg-[#eef8f3] px-3 py-2 text-xs text-[#165a4a] dark:border-[#315648] dark:bg-[#14251f] dark:text-[#aee4d0]"
              >
                Your saved working draft was restored after the latest immutable revision was checked.
              </p>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Title</span>
                <input v-model.trim="revisionForm.title" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Primary category</span>
                <select v-model="revisionForm.primaryCategoryId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option value="">Keep current category</option>
                  <option v-for="category in categories" :key="category.id" :value="category.id">{{ categoryPathLabel(category) }}</option>
                </select>
              </label>
              <RevisionContributorsEditor
                :model-value="revisionForm.contributors"
                :authors="authors"
                @update:model-value="updateRevisionContributors"
              />
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
              <fieldset class="grid gap-3 rounded-md border border-[#d7ded8] p-3 dark:border-[#3f4843] sm:grid-cols-2">
                <legend class="px-2 text-sm font-medium">SEO and social preview</legend>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">SEO title</span>
                  <input v-model.trim="revisionForm.seoTitle" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Defaults to revision title" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Robots</span>
                  <select v-model="revisionForm.robots" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                    <option value="index,follow">Index, follow</option>
                    <option value="index,nofollow">Index, nofollow</option>
                    <option value="noindex,follow">Noindex, follow</option>
                    <option value="noindex,nofollow">Noindex, nofollow</option>
                  </select>
                </label>
                <label class="block space-y-2 sm:col-span-2">
                  <span class="text-sm font-medium">Meta description</span>
                  <textarea v-model.trim="revisionForm.seoDescription" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Defaults to excerpt" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Open Graph title</span>
                  <input v-model.trim="revisionForm.openGraphTitle" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Open Graph image URL</span>
                  <input v-model.trim="revisionForm.openGraphImage" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="https://…" />
                </label>
                <label class="block space-y-2 sm:col-span-2">
                  <span class="text-sm font-medium">Open Graph description</span>
                  <textarea v-model.trim="revisionForm.openGraphDescription" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
              </fieldset>
              <ArticleStructuredEditor
                v-model:html="revisionForm.html"
                v-model:body-document="revisionBodyDocument"
                label="Revision body"
              />

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

            <form
              v-if="projectIsActive && copyDestinations.length"
              class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
              @submit.prevent="copyArticle"
            >
              <div class="flex items-start gap-3">
                <CopyPlus class="mt-1 h-4 w-4 text-[#6b5797]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Reuse</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Copy to project</h2>
                </div>
              </div>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Destination project</span>
                <select v-model="copyForm.destinationProjectId" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required>
                  <option value="">Select a project</option>
                  <option v-for="destination in copyDestinations" :key="destination.id" :value="destination.id">
                    {{ destination.name }}
                  </option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Source revision</span>
                <select v-model="copyForm.sourceRevisionId" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required>
                  <option v-for="revision in revisions" :key="revision.id" :value="revision.id">
                    #{{ revision.revisionNumber }} · {{ revision.title }}
                  </option>
                </select>
              </label>
              <fieldset v-if="copySourceContributors.length" class="space-y-3 rounded-md border border-[#d8d0e8] p-3 dark:border-[#4f4565]">
                <legend class="px-1 text-sm font-medium">Destination contributors</legend>
                <p class="text-xs text-[#5d6a61] dark:text-[#aeb8b0]">Explicitly map every credited source profile to an active profile owned by the destination project.</p>
                <label v-for="contributor in copySourceContributors" :key="contributor.authorId" class="block space-y-1">
                  <span class="text-xs font-medium">{{ contributor.name }} · {{ contributor.roles.join(', ') }}</span>
                  <select v-model="copyContributorMappings[contributor.authorId]" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required>
                    <option value="">Select destination author</option>
                    <option v-for="author in copyDestinationAuthors" :key="author.id" :value="author.id">{{ author.displayName }}</option>
                  </select>
                </label>
              </fieldset>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Destination category</span>
                <select
                  v-model="copyForm.primaryCategoryId"
                  class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
                  :disabled="loadingCopyCategories || !copyForm.destinationProjectId"
                  required
                >
                  <option value="">{{ loadingCopyCategories ? 'Loading categories…' : 'Select a category' }}</option>
                  <option v-for="category in copyDestinationCategories" :key="category.id" :value="category.id">
                    {{ categoryPathLabel(category) }}
                  </option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Destination slug</span>
                <input v-model.trim="copyForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Canonical decision</span>
                <select v-model="copyForm.canonicalDecision" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option value="material_adaptation">Material adaptation</option>
                  <option value="canonical_original">Canonical to original</option>
                </select>
              </label>
              <p
                v-if="copyForm.canonicalDecision === 'canonical_original'"
                class="rounded-md border border-[#d8d0e8] bg-[#f7f4fc] px-3 py-2 text-xs text-[#5e4b86] dark:border-[#4f4565] dark:bg-[#211d2a] dark:text-[#cbbfe2]"
              >
                The canonical URL is resolved by the server from the selected source revision. It cannot be redirected to another URL.
              </p>
              <p class="rounded-md bg-[#f2f5f3] px-3 py-2 text-xs text-[#5d6a61] dark:bg-[#171b18] dark:text-[#aeb8b0]">
                The destination gets a new unpublished draft and revision history. Choose destination taxonomy; copies containing project-owned body references are blocked until those references are removed or remapped.
              </p>

              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="copyingArticle || !canCopyArticle"
              >
                <LoaderCircle v-if="copyingArticle" class="h-4 w-4 animate-spin" />
                <CopyPlus v-else class="h-4 w-4" />
                Create destination draft
              </button>
            </form>

            <form v-if="canPublishArticles" class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="publishArticle">
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

            <form v-if="canPublishArticles" class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="scheduleArticle">
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

            <form v-if="canPublishArticles" class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="rollbackArticle">
              <div class="flex items-start gap-3">
                <History class="mt-1 h-4 w-4 text-[#6b5797]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Restore</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Rollback</h2>
                </div>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Approved revision</span>
                <select v-model="rollbackForm.revisionId" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" required>
                  <option value="">Select a revision</option>
                  <option v-for="revision in approvedRevisions" :key="revision.id" :value="revision.id" :disabled="isCurrentPublication(revision)">
                    #{{ revision.revisionNumber }} · {{ revision.title }}{{ isCurrentPublication(revision) ? ' (current)' : '' }}
                  </option>
                </select>
              </label>
              <p v-if="article.publicationState !== 'published'" class="text-xs text-[#667169] dark:text-[#aeb8b0]">Rollback is available while this article is published.</p>
              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="actionPending === 'rollback' || !canRollback"
              >
                <History class="h-4 w-4" />
                Rollback
              </button>
            </form>

            <section v-if="canPublishArticles" class="space-y-4 rounded-lg border border-[#d9b7aa] bg-white p-5 shadow-sm dark:border-[#6d352f] dark:bg-[#202522]">
              <div class="flex items-start gap-3">
                <Trash2 class="mt-1 h-4 w-4 text-[#9b2d23] dark:text-[#ffc4bd]" />
                <div>
                  <p class="text-sm text-[#9b2d23] dark:text-[#ffc4bd]">Danger zone</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Archive article</h2>
                </div>
              </div>
              <p class="text-sm text-[#5f6a63] dark:text-[#b8c2bb]">
                Hides this article from the admin list. If it is published, it is also removed from the Content API and downstream cache events are queued.
              </p>
              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-4 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                type="button"
                :disabled="actionPending === 'archive' || !canArchiveArticle"
                @click="archiveArticle"
              >
                <LoaderCircle v-if="actionPending === 'archive'" class="h-4 w-4 animate-spin" />
                <Trash2 v-else class="h-4 w-4" />
                Archive article
              </button>
            </section>
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
  CopyPlus,
  FilePenLine,
  GitCompareArrows,
  Hash,
  History,
  LoaderCircle,
  LogOut,
  MessageSquarePlus,
  Plus,
  RefreshCw,
  RotateCcw,
  Send,
  Trash2,
  UploadCloud,
  UserCheck,
  XCircle
} from 'lucide-vue-next'
import type { AdminAuthor, RevisionContributorInput } from '~/composables/useAdminApi'
import { articleBodyDocumentFromHTML, hasValidRevisionContributors, htmlToPlainText } from '~/composables/useAdminApi'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
  meta?: {
    nextCursor?: string
    limit: number
    openCount?: number
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
  editorialState: string
  contentHash: string
  createdAt: string
}

type AdminRevisionSummary = AdminRevision & {
  baseRevisionId?: string
  published: boolean
}

type AdminRevisionDetail = AdminRevisionSummary & {
  alternateTitle?: string
  bodyDocument: unknown
  tableOfContents: unknown
  authorSnapshot: unknown
  contributorSnapshot: unknown
  taxonomySnapshot: unknown
  sourceSnapshot: unknown
  claimSnapshot: unknown
  seoSnapshot: unknown
  socialSnapshot: unknown
  mediaSnapshot: unknown
  disclosureSnapshot: unknown
  correctionSummary: unknown
  sanitizedHtml: string
  plainText: string
  markdownExport: string
  wordCount: number
  readingTimeSeconds: number
  changeSummary?: string
}

type ComparisonField = {
  key: string
  label: string
  before: string
  after: string
  changed: boolean
  monospace?: boolean
  diffLines?: ComparisonDiffLine[]
}

type ComparisonDiffLine = {
  kind: 'equal' | 'added' | 'removed' | 'omitted'
  text: string
}

type ArticleDraftFields = {
  title: string
  primaryCategoryId: string
  contributors: RevisionContributorInput[]
  attributionEdited: boolean
  deck: string
  excerpt: string
  shortAnswer: string
  seoTitle: string
  seoDescription: string
  robots: string
  openGraphTitle: string
  openGraphDescription: string
  openGraphImage: string
  html: string
  bodyDocument: unknown
}

type ArticleDraftSnapshot = {
  schemaVersion: 3
  projectId: string
  articleId: string
  baseRevisionId: string
  savedAt: string
  fields: ArticleDraftFields
}

type ArticleAutosave = {
  projectId: string
  articleId: string
  userId: string
  baseRevisionId: string
  version: number
  draft: Omit<ArticleDraftFields, 'bodyDocument'> & { bodyDocument?: unknown }
  stale: boolean
  createdAt: string
  updatedAt: string
}

type ArticleDraftRecovery = {
  snapshot: ArticleDraftSnapshot
  source: 'browser' | 'server'
  reason: 'stale-base' | 'version-conflict'
}

type AdminArticle = {
  id: string
  projectId: string
  originProjectId?: string
  originArticleId?: string
  articleType: string
  slug: string
  title: string
  editorialState: string
  publicationState: string
  canonicalPolicy: string
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
  ancestors?: TaxonomyTerm[]
  children?: TaxonomyTerm[]
  indexable: boolean
}

type AdminProjectMember = {
  projectId: string
  userId: string
  email: string
  role: string
  status: string
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

type ReviewAssignment = {
  id: string
  projectId: string
  articleId: string
  revisionId?: string
  assignedTo: string
  assigneeEmail?: string
  assigneeRole?: string
  assignmentType: string
  dueAt?: string
  status: string
  createdBy: string
  createdAt: string
  closedBy?: string
  closedAt?: string
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
const currentUser = useState<{ id: string } | null>('admin-user', () => null)
const projects = ref<AdminProject[]>([])
const article = ref<AdminArticle | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const authors = ref<AdminAuthor[]>([])
const copyDestinationCategories = ref<TaxonomyTerm[]>([])
const copyDestinationAuthors = ref<AdminAuthor[]>([])
const copySourceRevision = ref<AdminRevisionDetail | null>(null)
const copyContributorMappings = reactive<Record<string, string>>({})
const members = ref<AdminProjectMember[]>([])
const comments = ref<ReviewComment[]>([])
const assignments = ref<ReviewAssignment[]>([])
const revisions = ref<AdminRevisionSummary[]>([])
const comparisonBefore = ref<AdminRevisionDetail | null>(null)
const comparisonAfter = ref<AdminRevisionDetail | null>(null)
const pending = ref(true)
const creatingRevision = ref(false)
const copyingArticle = ref(false)
const loadingCopyCategories = ref(false)
const creatingAssignment = ref(false)
const creatingComment = ref(false)
const loadingMoreAssignments = ref(false)
const loadingMoreComments = ref(false)
const loadingMoreRevisions = ref(false)
const comparisonPending = ref(false)
const nextAssignmentCursor = ref('')
const openAssignmentCount = ref(0)
const nextCommentCursor = ref('')
const nextRevisionCursor = ref('')
const actionPending = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const draftSaveState = ref<'idle' | 'saving' | 'saved' | 'restored'>('idle')
const draftSavedAt = ref('')
const serverDraftSaveState = ref<'idle' | 'saving' | 'saved' | 'restored' | 'conflict' | 'error'>('idle')
const serverDraftSavedAt = ref('')
const serverAutosaveVersion = ref(0)
const loadedServerAutosave = ref<ArticleAutosave | null>(null)
const staleDraft = ref<ArticleDraftRecovery | null>(null)
const reloadingServerDraft = ref(false)
const revisionBodyDocument = ref<unknown>({ type: 'doc', schemaVersion: 'tiptap-v1', content: [] })
const baseDraftRevisionID = ref('')
const attributionEdited = ref(false)
const commentPending = reactive<Record<string, string>>({})
const assignmentPending = reactive<Record<string, string>>({})
let comparisonRequestVersion = 0
let copyCategoryRequestVersion = 0
let copyContextRequestVersion = 0
let draftPersistenceEnabled = false
let draftDirty = false
let draftSaveTimer: ReturnType<typeof setTimeout> | undefined
let serverDraftDirty = false
let serverSaveInFlight = false
let serverSaveTimer: ReturnType<typeof setTimeout> | undefined
let serverSaveGeneration = 0

const revisionForm = reactive({
  title: '',
  primaryCategoryId: '',
  contributors: [] as RevisionContributorInput[],
  deck: '',
  excerpt: '',
  shortAnswer: '',
  seoTitle: '',
  seoDescription: '',
  robots: 'index,follow',
  openGraphTitle: '',
  openGraphDescription: '',
  openGraphImage: '',
  html: ''
})

const publicationForm = reactive({
  slug: '',
  canonicalUrl: ''
})

const rollbackForm = reactive({
  revisionId: ''
})

const copyForm = reactive({
  destinationProjectId: '',
  sourceRevisionId: '',
  primaryCategoryId: '',
  slug: '',
  canonicalDecision: 'material_adaptation'
})

const comparisonForm = reactive({
  beforeRevisionId: '',
  afterRevisionId: ''
})

const assignmentForm = reactive({
  revisionId: '',
  assignedTo: '',
  assignmentType: 'reviewer',
  dueAt: ''
})

const commentForm = reactive({
  revisionId: '',
  blockId: '',
  body: ''
})

const scheduleDraft = ref('')
const projectIsActive = computed(() => project.value?.status === 'active')
const canWriteArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'writer'].includes(project.value?.role || ''))
const canReviewArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'reviewer'].includes(project.value?.role || ''))
const canPublishArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
const canComment = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'reviewer', 'writer'].includes(project.value?.role || ''))
const canCreateRevision = computed(() => canWriteArticles.value && Boolean(
  revisionForm.title.trim()
  && hasValidRevisionContributors(revisionForm.contributors)
  && serverDraftSaveState.value !== 'conflict'
))
const draftStatusText = computed(() => {
  switch (serverDraftSaveState.value) {
    case 'saving':
      return 'Saving working draft to the server…'
    case 'saved':
    case 'restored':
      return serverDraftSavedAt.value ? `Saved to the server ${formatDate(serverDraftSavedAt.value)}` : 'Saved to the server'
    case 'conflict':
      return 'Server autosave paused · newer work exists in another tab'
    case 'error':
      return draftSavedAt.value
        ? `Server autosave unavailable · browser backup saved ${formatDate(draftSavedAt.value)}`
        : 'Server autosave unavailable · changes remain in this tab'
    default:
      if (draftSaveState.value === 'saving') return 'Saving a browser backup…'
      return draftSavedAt.value ? `Browser backup saved ${formatDate(draftSavedAt.value)}` : ''
  }
})
const draftStatusClass = computed(() => {
  if (serverDraftSaveState.value === 'conflict' || serverDraftSaveState.value === 'error') {
    return 'text-[#8a5b00] dark:text-[#ffd98a]'
  }
  return 'text-[#667169] dark:text-[#aeb8b0]'
})
const copyDestinations = computed(() => projects.value.filter(candidate =>
  candidate.id !== projectID.value
  && candidate.status === 'active'
  && ['project_owner', 'project_admin', 'editor', 'writer'].includes(candidate.role)
))
const canCopyArticle = computed(() => Boolean(
  projectIsActive.value
  && copyForm.destinationProjectId
  && copyForm.sourceRevisionId
  && copyForm.primaryCategoryId
  && copyForm.slug.trim()
  && ['canonical_original', 'material_adaptation'].includes(copyForm.canonicalDecision)
  && copySourceRevision.value
  && copySourceContributors.value.every(contributor => copyContributorMappings[contributor.authorId])
))
const copySourceContributors = computed(() => sourceContributorsFromRevision(copySourceRevision.value))
const openCommentCount = computed(() => comments.value.filter(comment => comment.status !== 'resolved').length)
const approvedRevisions = computed(() => revisions.value.filter(revision => revision.editorialState === 'approved'))
const canCompareRevisions = computed(() => Boolean(
  comparisonForm.beforeRevisionId
  && comparisonForm.afterRevisionId
  && comparisonForm.beforeRevisionId !== comparisonForm.afterRevisionId
))
const assignmentEligibleMembers = computed(() => members.value.filter(member =>
  member.status === 'active'
  && assignmentTypeAllowedForRole(assignmentForm.assignmentType, member.role)
))
const canManageAssignments = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
const canArchiveArticle = computed(() => canPublishArticles.value)
const canCreateAssignment = computed(() => Boolean(
  canManageAssignments.value
  && assignmentForm.assignedTo
  && assignmentEligibleMembers.value.some(member => member.userId === assignmentForm.assignedTo)
))
const canRollback = computed(() => {
  const selected = revisions.value.find(revision => revision.id === rollbackForm.revisionId)
  return Boolean(
    canPublishArticles.value
    && selected
    && selected.editorialState === 'approved'
    && article.value?.publicationState === 'published'
    && !isCurrentPublication(selected)
  )
})
const comparisonFields = computed<ComparisonField[]>(() => {
  if (!comparisonBefore.value || !comparisonAfter.value) return []
  const before = comparisonBefore.value
  const after = comparisonAfter.value
  return [
    comparisonField('title', 'Title', before.title, after.title),
    comparisonField('alternateTitle', 'Alternate title', before.alternateTitle || '', after.alternateTitle || ''),
    comparisonField('deck', 'Deck', before.deck || '', after.deck || ''),
    comparisonField('excerpt', 'Excerpt', before.excerpt || '', after.excerpt || ''),
    comparisonField('shortAnswer', 'Short answer', before.shortAnswer || '', after.shortAnswer || ''),
    comparisonField('bodyText', 'Body text', before.plainText, after.plainText, false, true),
    comparisonField('bodyDocument', 'Structured body', prettyJSON(before.bodyDocument), prettyJSON(after.bodyDocument), true, true),
    comparisonField('sanitizedHtml', 'Sanitized HTML', before.sanitizedHtml, after.sanitizedHtml, true, true),
    comparisonField('markdown', 'Markdown export', before.markdownExport, after.markdownExport, true, true),
    comparisonField('tableOfContents', 'Table of contents', prettyJSON(before.tableOfContents), prettyJSON(after.tableOfContents), true, true),
    comparisonField('authors', 'Authors', prettyJSON(before.authorSnapshot), prettyJSON(after.authorSnapshot), true, true),
    comparisonField('contributors', 'Contributors', prettyJSON(before.contributorSnapshot), prettyJSON(after.contributorSnapshot), true, true),
    comparisonField('taxonomy', 'Taxonomy snapshot', prettyJSON(before.taxonomySnapshot), prettyJSON(after.taxonomySnapshot), true, true),
    comparisonField('sources', 'Sources', prettyJSON(before.sourceSnapshot), prettyJSON(after.sourceSnapshot), true, true),
    comparisonField('claims', 'Claims', prettyJSON(before.claimSnapshot), prettyJSON(after.claimSnapshot), true, true),
    comparisonField('seo', 'SEO snapshot', prettyJSON(before.seoSnapshot), prettyJSON(after.seoSnapshot), true, true),
    comparisonField('social', 'Social snapshot', prettyJSON(before.socialSnapshot), prettyJSON(after.socialSnapshot), true, true),
    comparisonField('media', 'Media snapshot', prettyJSON(before.mediaSnapshot), prettyJSON(after.mediaSnapshot), true, true),
    comparisonField('disclosures', 'Disclosures', prettyJSON(before.disclosureSnapshot), prettyJSON(after.disclosureSnapshot), true, true),
    comparisonField('corrections', 'Corrections', prettyJSON(before.correctionSummary), prettyJSON(after.correctionSummary), true, true),
    comparisonField('wordCount', 'Word count', String(before.wordCount), String(after.wordCount)),
    comparisonField('readingTime', 'Reading time (seconds)', String(before.readingTimeSeconds), String(after.readingTimeSeconds)),
    comparisonField('changeSummary', 'Change summary', before.changeSummary || '', after.changeSummary || '')
  ]
})
const comparisonSummary = computed(() => {
  if (!comparisonBefore.value || !comparisonAfter.value) return ''
  const revisionSummary = `Revision ${comparisonBefore.value.revisionNumber} compared with revision ${comparisonAfter.value.revisionNumber}.`
  const changed = comparisonFields.value.filter(field => field.changed)
  if (changed.length === 0) {
    return `${revisionSummary} No public fields changed across ${comparisonFields.value.length} compared fields.`
  }
  const shownLabels = changed.slice(0, 6).map(field => field.label)
  const remaining = changed.length - shownLabels.length
  return `${revisionSummary} ${changed.length} of ${comparisonFields.value.length} fields changed: ${shownLabels.join(', ')}${remaining > 0 ? `, and ${remaining} more` : ''}.`
})

watch(
  () => [comparisonForm.beforeRevisionId, comparisonForm.afterRevisionId],
  () => invalidateComparison()
)

watch(
  () => [copyForm.destinationProjectId, copyForm.sourceRevisionId] as const,
  ([destinationProjectId, sourceRevisionId]) => loadCopyContext(destinationProjectId, sourceRevisionId)
)

watch(
  () => assignmentForm.assignmentType,
  () => {
    if (!assignmentEligibleMembers.value.some(member => member.userId === assignmentForm.assignedTo)) {
      assignmentForm.assignedTo = ''
    }
  }
)

watch(
  () => ({ ...revisionForm, bodyDocument: revisionBodyDocument.value }),
  () => {
    if (!draftPersistenceEnabled || !import.meta.client) return
    draftDirty = true
    serverDraftDirty = true
    draftSaveState.value = 'saving'
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = setTimeout(persistLocalDraft, 750)
    if (serverDraftSaveState.value !== 'conflict') {
      serverDraftSaveState.value = 'saving'
      queueServerAutosave()
    }
  },
  { deep: true }
)

onMounted(async () => {
  await refresh()
  restoreWorkingDraft()
})

onBeforeUnmount(() => {
  if (draftSaveTimer) clearTimeout(draftSaveTimer)
  if (serverSaveTimer) clearTimeout(serverSaveTimer)
  if (draftDirty) persistLocalDraft()
})

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, projectListResponse, memberResponse, categoryResponse, authorResponse, articleResponse, assignmentResponse, commentResponse, revisionResponse, autosaveResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      fetchAllCopyProjects(),
      fetchAllReviewAssignees(),
      fetchAllCategories(projectID.value),
      $fetch<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID.value}/authors`, { credentials: 'include' }),
      $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/assignments`, {
        credentials: 'include',
        query: { limit: 50 }
      }),
      $fetch<APIListEnvelope<ReviewComment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/comments`, {
        credentials: 'include',
        query: { limit: 50 }
      }),
      $fetch<APIListEnvelope<AdminRevisionSummary>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/revisions`, {
        credentials: 'include',
        query: { limit: 50 }
      }),
      fetchArticleAutosave()
    ])
    project.value = projectResponse.data
    projects.value = projectListResponse
    members.value = memberResponse
    categories.value = sortCategories(categoryResponse)
    authors.value = apiListData(authorResponse).sort((left, right) => left.displayName.localeCompare(right.displayName))
    setArticle(articleResponse.data)
    assignments.value = apiListData(assignmentResponse)
    nextAssignmentCursor.value = assignmentResponse.meta?.nextCursor || ''
    openAssignmentCount.value = assignmentResponse.meta?.openCount ?? assignments.value.filter(assignment => assignment.status === 'open').length
    comments.value = apiListData(commentResponse)
    nextCommentCursor.value = commentResponse.meta?.nextCursor || ''
    setRevisions(apiListData(revisionResponse), revisionResponse.meta?.nextCursor || '')
    loadedServerAutosave.value = autosaveResponse
    serverAutosaveVersion.value = autosaveResponse?.version || 0
    if (articleResponse.data.latestRevision?.id) {
      handleLatestRevisionDetail(await fetchRevisionDetail(articleResponse.data.latestRevision.id))
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load article. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function fetchArticleAutosave() {
  try {
    const response = await $fetch<APIEnvelope<ArticleAutosave>>(
      `/api/v1/projects/${projectID.value}/articles/${articleID.value}/autosave`,
      { credentials: 'include' }
    )
    return response.data
  } catch (error) {
    if (apiErrorStatus(error) === 404) return null
    throw error
  }
}

async function fetchAllCopyProjects() {
  const allProjects = new Map<string, AdminProject>()
  const seenCursors = new Set<string>()
  let cursor = ''

  do {
    const response = await $fetch<APIListEnvelope<AdminProject>>('/api/v1/projects', {
      credentials: 'include',
      query: {
        limit: 100,
        ...(cursor ? { cursor } : {})
      }
    })
    for (const candidate of apiListData(response)) allProjects.set(candidate.id, candidate)

    const nextCursor = response.meta?.nextCursor || ''
    if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Project pagination returned a repeated cursor')
    if (nextCursor) seenCursors.add(nextCursor)
    cursor = nextCursor
  } while (cursor)

  return [...allProjects.values()]
}

async function fetchAllReviewAssignees() {
  const allMembers = new Map<string, AdminProjectMember>()
  const seenCursors = new Set<string>()
  let cursor = ''

  try {
    do {
      const response = await $fetch<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID.value}/review-assignees`, {
        credentials: 'include',
        query: {
          limit: 100,
          ...(cursor ? { cursor } : {})
        }
      })
      for (const member of apiListData(response)) allMembers.set(member.userId, member)

      const nextCursor = response.meta?.nextCursor || ''
      if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Review-assignee pagination returned a repeated cursor')
      if (nextCursor) seenCursors.add(nextCursor)
      cursor = nextCursor
    } while (cursor)
  } catch (error) {
    if (apiErrorStatus(error) === 403) return []
    throw error
  }

  return [...allMembers.values()]
}

async function createRevision() {
  if (!article.value) return
  creatingRevision.value = true
  clearMessages()
  if (serverSaveTimer) {
    clearTimeout(serverSaveTimer)
    serverSaveTimer = undefined
  }
  try {
    const csrfToken = await getCSRFToken()
    const html = effectiveDraftHTML()
    const response = await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/articles/${article.value.id}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        baseRevisionId: latestRevisionID(),
        title: revisionForm.title,
        primaryCategoryId: revisionForm.primaryCategoryId,
        ...(attributionEdited.value ? { contributors: revisionForm.contributors } : {}),
        deck: revisionForm.deck,
        excerpt: revisionForm.excerpt,
        shortAnswer: revisionForm.shortAnswer,
        bodyDocument: draftBodyDocument(),
        html,
        seo: {
          title: revisionForm.seoTitle,
          description: revisionForm.seoDescription,
          robots: revisionForm.robots,
          openGraphTitle: revisionForm.openGraphTitle,
          openGraphDescription: revisionForm.openGraphDescription,
          openGraphImage: revisionForm.openGraphImage
        }
      }
    })
    serverSaveGeneration += 1
    draftPersistenceEnabled = false
    if (serverSaveTimer) {
      clearTimeout(serverSaveTimer)
      serverSaveTimer = undefined
    }
    removeLocalDraft()
    successMessage.value = `Revision #${response.data.revisionNumber} created.`
    await fetchArticle()
    setRevisionDraftFromDetail(await fetchRevisionDetail(response.data.id))
    await nextTick()
    draftDirty = false
    serverDraftDirty = false
    draftSaveState.value = 'idle'
    draftSavedAt.value = ''
    serverDraftSaveState.value = 'idle'
    serverDraftSavedAt.value = ''
    serverAutosaveVersion.value = 0
    loadedServerAutosave.value = null
    staleDraft.value = null
    draftPersistenceEnabled = true
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create revision.')
    serverDraftDirty = true
    if (serverDraftSaveState.value !== 'conflict') {
      serverDraftSaveState.value = 'saving'
      queueServerAutosave(100)
    }
  } finally {
    creatingRevision.value = false
  }
}

async function copyArticle() {
  if (!article.value || !canCopyArticle.value) return
  copyingArticle.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminArticle>>(
      `/api/v1/projects/${projectID.value}/articles/${article.value.id}/copy-to-project`,
      {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body: {
          destinationProjectId: copyForm.destinationProjectId,
          sourceRevisionId: copyForm.sourceRevisionId,
          primaryCategoryId: copyForm.primaryCategoryId,
          slug: copyForm.slug,
          canonicalDecision: copyForm.canonicalDecision,
          contributorMappings: copySourceContributors.value.map(contributor => ({
            sourceAuthorId: contributor.authorId,
            destinationAuthorId: copyContributorMappings[contributor.authorId]
          }))
        }
      }
    )
    await navigateTo(`/projects/${response.data.projectId}/articles/${response.data.id}`)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not copy article.')
  } finally {
    copyingArticle.value = false
  }
}

async function loadCopyDestinationCategories(destinationProjectId: string) {
  const requestVersion = ++copyCategoryRequestVersion
  copyDestinationCategories.value = []
  copyForm.primaryCategoryId = ''
  if (!destinationProjectId) return
  loadingCopyCategories.value = true
  try {
    const allCategories = await fetchAllCategories(destinationProjectId)
    if (requestVersion !== copyCategoryRequestVersion) return
    copyDestinationCategories.value = sortCategories(allCategories)
  } catch (error) {
    if (requestVersion !== copyCategoryRequestVersion) return
    errorMessage.value = normalizeAPIError(error, 'Could not load destination categories.')
  } finally {
    if (requestVersion === copyCategoryRequestVersion) {
      loadingCopyCategories.value = false
    }
  }
}

async function loadCopyContext(destinationProjectId: string, sourceRevisionId: string) {
  const requestVersion = ++copyContextRequestVersion
  copyDestinationAuthors.value = []
  copySourceRevision.value = null
  for (const key of Object.keys(copyContributorMappings)) delete copyContributorMappings[key]
  await loadCopyDestinationCategories(destinationProjectId)
  if (!destinationProjectId || !sourceRevisionId || requestVersion !== copyContextRequestVersion) return
  try {
    const [authorResponse, revision] = await Promise.all([
      fetchAllAuthors(destinationProjectId),
      fetchRevisionDetail(sourceRevisionId)
    ])
    if (requestVersion !== copyContextRequestVersion) return
    copyDestinationAuthors.value = authorResponse
      .filter(author => author.status === 'active')
      .sort((left, right) => left.displayName.localeCompare(right.displayName))
    copySourceRevision.value = revision
  } catch (error) {
    if (requestVersion === copyContextRequestVersion) errorMessage.value = normalizeAPIError(error, 'Could not load contributor mappings.')
  }
}

async function fetchAllAuthors(targetProjectID: string) {
  const allAuthors = new Map<string, AdminAuthor>()
  const seenCursors = new Set<string>()
  let cursor = ''
  do {
    const response = await $fetch<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${targetProjectID}/authors`, {
      credentials: 'include',
      query: { limit: 100, ...(cursor ? { cursor } : {}) }
    })
    for (const author of apiListData(response)) allAuthors.set(author.id, author)
    const nextCursor = response.meta?.nextCursor || ''
    if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Author pagination returned a repeated cursor')
    if (nextCursor) seenCursors.add(nextCursor)
    cursor = nextCursor
  } while (cursor)
  return [...allAuthors.values()]
}

function sourceContributorsFromRevision(revision: AdminRevisionDetail | null) {
  if (!revision) return [] as Array<{ authorId: string, name: string, roles: string[] }>
  const values = new Map<string, { authorId: string, name: string, roles: string[] }>()
  const authors = Array.isArray(revision.authorSnapshot) ? revision.authorSnapshot : []
  authors.forEach((value, index) => addSourceContributor(values, value, index === 0 ? 'primary author' : 'co-author'))
  const contributors = Array.isArray(revision.contributorSnapshot) ? revision.contributorSnapshot : []
  contributors.forEach((value) => {
    if (!value || typeof value !== 'object') return
    const record = value as Record<string, unknown>
    addSourceContributor(values, record.author, String(record.role || 'contributor').replaceAll('_', ' '))
  })
  return [...values.values()]
}

function addSourceContributor(target: Map<string, { authorId: string, name: string, roles: string[] }>, value: unknown, role: string) {
  if (!value || typeof value !== 'object') return
  const author = value as Record<string, unknown>
  const authorId = typeof author.id === 'string' ? author.id : ''
  if (!authorId) return
  const existing = target.get(authorId)
  if (existing) {
    if (!existing.roles.includes(role)) existing.roles.push(role)
    return
  }
  target.set(authorId, { authorId, name: typeof author.displayName === 'string' ? author.displayName : authorId, roles: [role] })
}

async function fetchAllCategories(targetProjectID: string) {
  const allCategories = new Map<string, TaxonomyTerm>()
  const seenCursors = new Set<string>()
  let cursor = ''

  do {
    const response = await $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${targetProjectID}/categories`, {
      credentials: 'include',
      query: { limit: 100, ...(cursor ? { cursor } : {}) }
    })
    for (const category of apiListData(response)) allCategories.set(category.id, category)
    const nextCursor = response.meta?.nextCursor || ''
    if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Category pagination returned a repeated cursor')
    if (nextCursor) seenCursors.add(nextCursor)
    cursor = nextCursor
  } while (cursor)

  return [...allCategories.values()]
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
  const selected = revisions.value.find(revision => revision.id === rollbackForm.revisionId)
  if (!selected || !canRollback.value) return
  if (!window.confirm(`Rollback this article to revision #${selected.revisionNumber}?`)) return
  await mutateArticle('rollback', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/rollback`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: { revisionId: rollbackForm.revisionId }
    })
    rollbackForm.revisionId = ''
    successMessage.value = 'Article rolled back.'
  })
}

async function archiveArticle() {
  if (!article.value || !canArchiveArticle.value) return
  const message = article.value.publicationState === 'published'
    ? `Archive "${article.value.title}"? This will unpublish it from the content API and hide it from the admin article list.`
    : `Archive "${article.value.title}"? This will hide it from the admin article list while retaining its revision history.`
  if (!window.confirm(message)) return
  actionPending.value = 'archive'
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    await $fetch(`/api/v1/projects/${projectID.value}/articles/${article.value.id}`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
    await navigateTo(`/projects/${projectID.value}/articles`)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not archive article.')
    actionPending.value = ''
  }
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

async function createAssignment() {
  if (!canCreateAssignment.value) return
  creatingAssignment.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/assignments`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        revisionId: assignmentForm.revisionId || undefined,
        assignedTo: assignmentForm.assignedTo,
        assignmentType: assignmentForm.assignmentType,
        dueAt: assignmentDueAtForAPI()
      }
    })
    assignments.value = [response.data, ...assignments.value]
    openAssignmentCount.value += 1
    assignmentForm.assignedTo = ''
    successMessage.value = 'Assignment created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create assignment.')
  } finally {
    creatingAssignment.value = false
  }
}

async function setAssignmentStatus(assignment: ReviewAssignment, action: 'complete' | 'cancel') {
  if (action === 'complete' ? !canCompleteAssignment(assignment) : !canManageAssignments.value) return
  assignmentPending[assignment.id] = action
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID.value}/assignments/${assignment.id}/${action}`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    assignments.value = assignments.value.map(candidate => candidate.id === response.data.id ? response.data : candidate)
    if (assignment.status === 'open' && response.data.status !== 'open') {
      openAssignmentCount.value = Math.max(0, openAssignmentCount.value - 1)
    }
    successMessage.value = action === 'complete' ? 'Assignment completed.' : 'Assignment cancelled.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, action === 'complete' ? 'Could not complete assignment.' : 'Could not cancel assignment.')
  } finally {
    delete assignmentPending[assignment.id]
  }
}

async function createComment() {
  if (!canComment.value || !commentForm.body.trim()) return
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
  if (!canComment.value) return
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

async function loadMoreAssignments() {
  if (!nextAssignmentCursor.value || loadingMoreAssignments.value) return
  loadingMoreAssignments.value = true
  clearMessages()
  try {
    const response = await $fetch<APIListEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/assignments`, {
      credentials: 'include',
      query: {
        cursor: nextAssignmentCursor.value,
        limit: 50
      }
    })
    const merged = new Map(assignments.value.map(assignment => [assignment.id, assignment]))
    for (const assignment of apiListData(response)) merged.set(assignment.id, assignment)
    assignments.value = [...merged.values()]
    nextAssignmentCursor.value = response.meta?.nextCursor || ''
    if (response.meta?.openCount !== undefined) openAssignmentCount.value = response.meta.openCount
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more assignments.')
  } finally {
    loadingMoreAssignments.value = false
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
    for (const comment of apiListData(response)) merged.set(comment.id, comment)
    comments.value = [...merged.values()]
    nextCommentCursor.value = response.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more comments.')
  } finally {
    loadingMoreComments.value = false
  }
}

async function loadMoreRevisions() {
  if (!nextRevisionCursor.value || loadingMoreRevisions.value) return
  loadingMoreRevisions.value = true
  clearMessages()
  try {
    const response = await $fetch<APIListEnvelope<AdminRevisionSummary>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/revisions`, {
      credentials: 'include',
      query: {
        cursor: nextRevisionCursor.value,
        limit: 50
      }
    })
    const merged = new Map(revisions.value.map(revision => [revision.id, revision]))
    for (const revision of apiListData(response)) merged.set(revision.id, revision)
    setRevisions([...merged.values()].sort((left, right) => right.revisionNumber - left.revisionNumber), response.meta?.nextCursor || '')
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load older revisions.')
  } finally {
    loadingMoreRevisions.value = false
  }
}

function selectRevisionForComparison(side: 'before' | 'after', revisionId: string) {
  if (comparisonPending.value) return
  if (side === 'before') {
    comparisonForm.beforeRevisionId = revisionId
  } else {
    comparisonForm.afterRevisionId = revisionId
  }
}

async function compareRevisions() {
  if (!canCompareRevisions.value) return
  const requestedBeforeID = comparisonForm.beforeRevisionId
  const requestedAfterID = comparisonForm.afterRevisionId
  const requestVersion = ++comparisonRequestVersion
  comparisonPending.value = true
  comparisonBefore.value = null
  comparisonAfter.value = null
  clearMessages()
  try {
    const [beforeResponse, afterResponse] = await Promise.all([
      fetchRevisionDetail(requestedBeforeID),
      fetchRevisionDetail(requestedAfterID)
    ])
    if (requestVersion !== comparisonRequestVersion) return
    if (beforeResponse.revisionNumber <= afterResponse.revisionNumber) {
      comparisonBefore.value = beforeResponse
      comparisonAfter.value = afterResponse
    } else {
      comparisonBefore.value = afterResponse
      comparisonAfter.value = beforeResponse
    }
  } catch (error) {
    if (requestVersion !== comparisonRequestVersion) return
    errorMessage.value = normalizeAPIError(error, 'Could not compare revisions.')
  } finally {
    if (requestVersion === comparisonRequestVersion) {
      comparisonPending.value = false
    }
  }
}

async function fetchRevisionDetail(revisionId: string) {
  const response = await $fetch<APIEnvelope<AdminRevisionDetail>>(
    `/api/v1/projects/${projectID.value}/articles/${articleID.value}/revisions/${revisionId}`,
    { credentials: 'include' }
  )
  return response.data
}

async function fetchArticle() {
  const [articleResponse, revisionResponse] = await Promise.all([
    $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}`, {
      credentials: 'include'
    }),
    $fetch<APIListEnvelope<AdminRevisionSummary>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/revisions`, {
      credentials: 'include',
      query: { limit: 50 }
    })
  ])
  setArticle(articleResponse.data)
  setRevisions(apiListData(revisionResponse), revisionResponse.meta?.nextCursor || '')
}

function setArticle(value: AdminArticle) {
  article.value = value
  publicationForm.slug = value.slug
  publicationForm.canonicalUrl = value.canonicalUrl || ''
  revisionForm.title = revisionForm.title || value.title
  copyForm.sourceRevisionId = copyForm.sourceRevisionId || value.latestRevision?.id || ''
  copyForm.slug = copyForm.slug || `${value.slug}-copy`
  assignmentForm.revisionId = value.latestRevision?.id || ''
  commentForm.revisionId = value.latestRevision?.id || ''
  if (!scheduleDraft.value) {
    const scheduledAt = value.scheduledForUtc ? parseBackendUTC(value.scheduledForUtc) : new Date(Date.now() + 15 * 60 * 1000)
    scheduleDraft.value = toLocalInputValue(scheduledAt)
  }
}

function setRevisions(values: AdminRevisionSummary[], nextCursor: string) {
  revisions.value = values
  nextRevisionCursor.value = nextCursor

  const latest = values[0]
  if (!latest) {
    comparisonForm.beforeRevisionId = ''
    comparisonForm.afterRevisionId = ''
    return
  }
  if (!values.some(revision => revision.id === comparisonForm.afterRevisionId)) {
    comparisonForm.afterRevisionId = latest.id
  }
  if (!values.some(revision => revision.id === copyForm.sourceRevisionId)) {
    copyForm.sourceRevisionId = latest.id
  }
  if (!values.some(revision => revision.id === comparisonForm.beforeRevisionId)) {
    comparisonForm.beforeRevisionId = latest.baseRevisionId || values[1]?.id || ''
  }
  const rollbackRevision = values.find(revision => revision.id === rollbackForm.revisionId)
  if (!rollbackRevision || isCurrentPublication(rollbackRevision)) {
    rollbackForm.revisionId = ''
  }
}

function setRevisionDraftFromDetail(revision: AdminRevisionDetail) {
  const seo = seoFieldsFromSnapshot(revision.seoSnapshot)
  revisionForm.title = revision.title
  revisionForm.primaryCategoryId = primaryCategoryIDFromSnapshot(revision.taxonomySnapshot)
  revisionForm.contributors = contributorInputsFromSnapshots(revision.authorSnapshot, revision.contributorSnapshot)
  attributionEdited.value = false
  revisionForm.deck = revision.deck || ''
  revisionForm.excerpt = revision.excerpt || ''
  revisionForm.shortAnswer = revision.shortAnswer || ''
  revisionForm.seoTitle = seo.title
  revisionForm.seoDescription = seo.description
  revisionForm.robots = seo.robots
  revisionForm.openGraphTitle = seo.openGraphTitle
  revisionForm.openGraphDescription = seo.openGraphDescription
  revisionForm.openGraphImage = seo.openGraphImage
  revisionForm.html = revision.sanitizedHtml || `<p>${escapeHTML(revision.title)}</p>`
  revisionBodyDocument.value = isStructuredBodyDocument(revision.bodyDocument)
    ? revision.bodyDocument
    : articleBodyDocumentFromHTML(revisionForm.html, revisionForm.title)
  baseDraftRevisionID.value = revision.id
}

function handleLatestRevisionDetail(revision: AdminRevisionDetail) {
  if (!draftPersistenceEnabled || !baseDraftRevisionID.value) {
    setRevisionDraftFromDetail(revision)
    return
  }
  if (revision.id === baseDraftRevisionID.value) return

  if (draftDirty) persistLocalDraft()
  const localDraft = readLocalDraft()
  if (serverSaveTimer) {
    clearTimeout(serverSaveTimer)
    serverSaveTimer = undefined
  }
  draftPersistenceEnabled = false
  setRevisionDraftFromDetail(revision)
  draftDirty = false
  serverDraftDirty = false
  draftSaveState.value = 'idle'
  draftSavedAt.value = ''
  serverDraftSaveState.value = 'idle'
  serverDraftSavedAt.value = ''
  const recoveries: ArticleDraftRecovery[] = []
  if (localDraft && localDraft.baseRevisionId !== revision.id) {
    recoveries.push({ snapshot: localDraft, source: 'browser', reason: 'stale-base' })
  }
  if (loadedServerAutosave.value && loadedServerAutosave.value.baseRevisionId !== revision.id) {
    recoveries.push(articleAutosaveRecovery(loadedServerAutosave.value))
  }
  staleDraft.value = newestDraftRecovery(recoveries)
  nextTick(() => {
    draftPersistenceEnabled = true
  })
}

function seoFieldsFromSnapshot(snapshot: unknown) {
  const fallback = {
    title: '',
    description: '',
    robots: 'index,follow',
    openGraphTitle: '',
    openGraphDescription: '',
    openGraphImage: ''
  }
  if (!snapshot || typeof snapshot !== 'object') return fallback
  const seo = snapshot as Record<string, unknown>
  const openGraph = seo.openGraph && typeof seo.openGraph === 'object'
    ? seo.openGraph as Record<string, unknown>
    : {}
  return {
    title: typeof seo.title === 'string' ? seo.title : '',
    description: typeof seo.description === 'string' ? seo.description : '',
    robots: typeof seo.robots === 'string' ? seo.robots : 'index,follow',
    openGraphTitle: typeof openGraph.title === 'string' ? openGraph.title : '',
    openGraphDescription: typeof openGraph.description === 'string' ? openGraph.description : '',
    openGraphImage: typeof openGraph.image === 'string' ? openGraph.image : ''
  }
}

function primaryCategoryIDFromSnapshot(snapshot: unknown) {
  if (!snapshot || typeof snapshot !== 'object') return ''
  const taxonomy = snapshot as { primaryCategory?: { id?: unknown } }
  return typeof taxonomy.primaryCategory?.id === 'string' ? taxonomy.primaryCategory.id : ''
}

function contributorInputsFromSnapshots(authorSnapshot: unknown, contributorSnapshot: unknown): RevisionContributorInput[] {
  const authorInputs = Array.isArray(authorSnapshot)
    ? authorSnapshot.flatMap((value, index): RevisionContributorInput[] => {
        if (!value || typeof value !== 'object') return []
        const authorID = (value as Record<string, unknown>).id
        if (typeof authorID !== 'string' || !authorID) return []
        return [{
          authorId: authorID,
          role: index === 0 ? 'primary_author' : 'co_author',
          position: index === 0 ? 0 : index - 1
        }]
      })
    : []
  const creditedInputs = Array.isArray(contributorSnapshot)
    ? contributorSnapshot.flatMap((value): RevisionContributorInput[] => {
        if (!value || typeof value !== 'object') return []
        const snapshot = value as Record<string, unknown>
        const author = snapshot.author && typeof snapshot.author === 'object'
          ? snapshot.author as Record<string, unknown>
          : {}
        if (typeof author.id !== 'string' || !isContributorRole(snapshot.role)) return []
        return [{
          authorId: author.id,
          role: snapshot.role,
          position: typeof snapshot.position === 'number' ? snapshot.position : 0
        }]
      })
    : []
  return [...authorInputs, ...creditedInputs]
}

function isContributorRole(value: unknown): value is RevisionContributorInput['role'] {
  return ['primary_author', 'co_author', 'editor', 'expert_reviewer', 'photographer', 'other'].includes(String(value))
}

function updateRevisionContributors(value: RevisionContributorInput[]) {
  revisionForm.contributors = value
  attributionEdited.value = true
}

function categoryPathLabel(category: TaxonomyTerm) {
  return [...(category.ancestors || []).map(ancestor => ancestor.name), category.name].join(' / ')
}

function publicationBody(revisionId: string) {
  return {
    revisionId,
    slug: publicationForm.slug,
    canonicalUrl: publicationForm.canonicalUrl || undefined
  }
}

function latestRevisionID() {
  return article.value?.latestRevision?.id || ''
}

function localDraftKey() {
  return `seoblog:article-draft:${projectID.value}:${articleID.value}`
}

function draftFieldsSnapshot(): ArticleDraftFields {
  return {
    ...revisionForm,
    contributors: revisionForm.contributors.map(contributor => ({ ...contributor })),
    attributionEdited: attributionEdited.value,
    bodyDocument: draftBodyDocument()
  }
}

function draftSnapshot(savedAt = new Date().toISOString()): ArticleDraftSnapshot {
  return {
    schemaVersion: 3,
    projectId: projectID.value,
    articleId: articleID.value,
    baseRevisionId: baseDraftRevisionID.value || latestRevisionID(),
    savedAt,
    fields: draftFieldsSnapshot()
  }
}

function effectiveDraftHTML() {
  return hasMeaningfulStructuredHTML(revisionForm.html)
    ? revisionForm.html.trim()
    : `<p>${escapeHTML(revisionForm.title)}</p>`
}

function draftBodyDocument() {
  return hasMeaningfulStructuredHTML(revisionForm.html) && isStructuredBodyDocument(revisionBodyDocument.value)
    ? revisionBodyDocument.value
    : articleBodyDocumentFromHTML(effectiveDraftHTML(), revisionForm.title)
}

function persistLocalDraft() {
  if (!import.meta.client || !draftPersistenceEnabled || !draftDirty) return
  if (draftSaveTimer) {
    clearTimeout(draftSaveTimer)
    draftSaveTimer = undefined
  }
  const snapshot = draftSnapshot()
  if (writeLocalDraft(snapshot)) {
    draftDirty = false
    draftSavedAt.value = snapshot.savedAt
    draftSaveState.value = 'saved'
  } else {
    draftSaveState.value = 'idle'
    errorMessage.value = 'This browser could not save the revision draft locally.'
  }
}

function queueServerAutosave(delay = 1500) {
  if (!import.meta.client || !canWriteArticles.value || serverDraftSaveState.value === 'conflict') return
  if (serverSaveTimer) clearTimeout(serverSaveTimer)
  serverSaveTimer = setTimeout(persistServerAutosave, delay)
}

async function persistServerAutosave() {
  if (serverSaveInFlight) {
    queueServerAutosave(250)
    return
  }
  if (
    !import.meta.client
    || !draftPersistenceEnabled
    || !canWriteArticles.value
    || !serverDraftDirty
    || serverDraftSaveState.value === 'conflict'
  ) return
  const baseRevisionId = baseDraftRevisionID.value || latestRevisionID()
  if (!baseRevisionId) return
  const saveGeneration = serverSaveGeneration
  if (serverSaveTimer) {
    clearTimeout(serverSaveTimer)
    serverSaveTimer = undefined
  }

  serverSaveInFlight = true
  serverDraftDirty = false
  serverDraftSaveState.value = 'saving'
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ArticleAutosave>>(
      `/api/v1/projects/${projectID.value}/articles/${articleID.value}/autosave`,
      {
        method: 'PUT',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body: {
          baseRevisionId,
          expectedVersion: serverAutosaveVersion.value,
          draft: draftFieldsSnapshot()
        }
      }
    )
    if (saveGeneration !== serverSaveGeneration) return
    loadedServerAutosave.value = response.data
    serverAutosaveVersion.value = response.data.version
    serverDraftSavedAt.value = response.data.updatedAt
    serverDraftSaveState.value = 'saved'
    if (staleDraft.value?.source === 'server') staleDraft.value = null
    const synchronizedSnapshot = articleAutosaveSnapshot(response.data)
    if (!serverDraftDirty && staleDraft.value?.source !== 'browser' && writeLocalDraft(synchronizedSnapshot)) {
      draftDirty = false
      draftSavedAt.value = synchronizedSnapshot.savedAt
      draftSaveState.value = 'saved'
    }
  } catch (error) {
    if (saveGeneration !== serverSaveGeneration) return
    serverDraftDirty = true
    serverDraftSaveState.value = apiErrorStatus(error) === 409 ? 'conflict' : 'error'
  } finally {
    serverSaveInFlight = false
    if (saveGeneration === serverSaveGeneration && serverDraftDirty && serverDraftSaveState.value !== 'conflict' && serverDraftSaveState.value !== 'error') {
      queueServerAutosave(250)
    }
  }
}

async function restoreWorkingDraft() {
  if (!import.meta.client) return
  const recoveries: ArticleDraftRecovery[] = []
  const localSnapshot = readLocalDraft()
  if (localSnapshot) {
    recoveries.push({ snapshot: localSnapshot, source: 'browser', reason: 'stale-base' })
  }
  if (loadedServerAutosave.value) {
    recoveries.push(articleAutosaveRecovery(loadedServerAutosave.value))
  }

  const currentRecoveries = recoveries.filter(recovery => recovery.snapshot.baseRevisionId === latestRevisionID())
  const staleRecoveries = recoveries.filter(recovery => recovery.snapshot.baseRevisionId !== latestRevisionID())
  staleDraft.value = newestDraftRecovery(staleRecoveries)
  const currentRecovery = newestDraftRecovery(currentRecoveries)
  if (!currentRecovery) {
    draftPersistenceEnabled = true
    return
  }

  await applyDraftRecovery(currentRecovery)
  if (currentRecovery.source === 'server') {
    serverDraftSavedAt.value = currentRecovery.snapshot.savedAt
    serverDraftSaveState.value = 'restored'
    if (staleDraft.value?.source !== 'browser') writeLocalDraft(currentRecovery.snapshot)
  } else {
    draftSavedAt.value = currentRecovery.snapshot.savedAt
    draftSaveState.value = 'restored'
    const serverSavedAt = loadedServerAutosave.value?.updatedAt || ''
    if (!serverSavedAt || recoveryTimestamp(currentRecovery.snapshot.savedAt) > recoveryTimestamp(serverSavedAt)) {
      serverDraftDirty = true
      serverDraftSaveState.value = 'saving'
      queueServerAutosave(100)
    }
  }
}

async function restoreStaleDraft() {
  if (!staleDraft.value) return
  const recovery = staleDraft.value
  staleDraft.value = null
  await applyDraftRecovery(recovery)
  draftDirty = true
  serverDraftDirty = true
  serverDraftSaveState.value = 'saving'
  persistLocalDraft()
  queueServerAutosave(100)
}

async function discardLocalDraft() {
  const recovery = staleDraft.value
  if (!recovery) return
  const browserRecovery = readLocalDraft()
  try {
    if (recovery.source === 'server') {
      await deleteServerAutosave()
    } else {
      removeLocalDraft()
      draftSavedAt.value = ''
      draftSaveState.value = 'idle'
    }
    if (
      recovery.source === 'server'
      && browserRecovery
      && browserRecovery.baseRevisionId !== latestRevisionID()
    ) {
      staleDraft.value = { snapshot: browserRecovery, source: 'browser', reason: 'stale-base' }
    } else if (
      recovery.source === 'browser'
      && loadedServerAutosave.value
      && loadedServerAutosave.value.baseRevisionId !== latestRevisionID()
    ) {
      staleDraft.value = articleAutosaveRecovery(loadedServerAutosave.value)
    } else {
      staleDraft.value = null
    }
    if (
      recovery.source === 'browser'
      && !staleDraft.value
      && loadedServerAutosave.value?.baseRevisionId === latestRevisionID()
    ) {
      const currentServerSnapshot = articleAutosaveSnapshot(loadedServerAutosave.value)
      if (writeLocalDraft(currentServerSnapshot)) {
        draftSavedAt.value = currentServerSnapshot.savedAt
        draftSaveState.value = 'saved'
      }
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not discard the saved working draft.')
  }
}

async function applyDraftRecovery(recovery: ArticleDraftRecovery) {
  draftPersistenceEnabled = false
  const snapshot = recovery.snapshot
  const { attributionEdited: savedAttributionEdited, bodyDocument, ...fields } = snapshot.fields
  Object.assign(revisionForm, fields)
  attributionEdited.value = savedAttributionEdited
  revisionBodyDocument.value = bodyDocument
  draftSavedAt.value = snapshot.savedAt
  await nextTick()
  draftPersistenceEnabled = true
}

function articleAutosaveSnapshot(autosave: ArticleAutosave): ArticleDraftSnapshot {
  const bodyDocument = isStructuredBodyDocument(autosave.draft.bodyDocument)
    ? autosave.draft.bodyDocument
    : articleBodyDocumentFromHTML(autosave.draft.html, autosave.draft.title)
  return {
    schemaVersion: 3,
    projectId: autosave.projectId,
    articleId: autosave.articleId,
    baseRevisionId: autosave.baseRevisionId,
    savedAt: autosave.updatedAt,
    fields: {
      ...autosave.draft,
      bodyDocument,
      contributors: autosave.draft.contributors.map(contributor => ({ ...contributor }))
    }
  }
}

function articleAutosaveRecovery(autosave: ArticleAutosave): ArticleDraftRecovery {
  return {
    snapshot: articleAutosaveSnapshot(autosave),
    source: 'server',
    reason: 'stale-base'
  }
}

function newestDraftRecovery(recoveries: ArticleDraftRecovery[]) {
  return recoveries.reduce<ArticleDraftRecovery | null>((newest, candidate) => {
    if (!newest) return candidate
    const candidateTime = recoveryTimestamp(candidate.snapshot.savedAt)
    const newestTime = recoveryTimestamp(newest.snapshot.savedAt)
    if (candidateTime > newestTime) {
      return candidate
    }
    if (candidateTime === newestTime) {
      const fieldsMatch = JSON.stringify(candidate.snapshot.fields) === JSON.stringify(newest.snapshot.fields)
      if (fieldsMatch && candidate.source === 'server') return candidate
      if (!fieldsMatch && candidate.source === 'browser') return candidate
    }
    return newest
  }, null)
}

function recoveryTimestamp(value: string) {
  const timestamp = parseBackendUTC(value).getTime()
  return Number.isNaN(timestamp) ? 0 : timestamp
}

async function reloadServerDraft() {
  if (reloadingServerDraft.value) return
  reloadingServerDraft.value = true
  if (draftDirty) persistLocalDraft()
  const browserBackup = readLocalDraft()
  try {
    const autosave = await fetchArticleAutosave()
    if (!autosave) {
      serverAutosaveVersion.value = 0
      loadedServerAutosave.value = null
      serverDraftSaveState.value = 'error'
      return
    }
    loadedServerAutosave.value = autosave
    serverAutosaveVersion.value = autosave.version
    serverDraftDirty = false
    const recovery = articleAutosaveRecovery(autosave)
    if (autosave.stale || recovery.snapshot.baseRevisionId !== latestRevisionID()) {
      staleDraft.value = recovery
      serverDraftSaveState.value = 'idle'
      return
    }
    await applyDraftRecovery(recovery)
    serverDraftSavedAt.value = autosave.updatedAt
    serverDraftSaveState.value = 'restored'
    if (browserBackup && JSON.stringify(browserBackup.fields) !== JSON.stringify(recovery.snapshot.fields)) {
      staleDraft.value = {
        snapshot: browserBackup,
        source: 'browser',
        reason: 'version-conflict'
      }
    }
  } catch (error) {
    serverDraftSaveState.value = 'error'
    errorMessage.value = normalizeAPIError(error, 'Could not reload the newer server draft.')
  } finally {
    reloadingServerDraft.value = false
  }
}

async function deleteServerAutosave() {
  const csrfToken = await getCSRFToken()
  await $fetch(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/autosave`, {
    method: 'DELETE',
    credentials: 'include',
    headers: { 'X-CSRF-Token': csrfToken }
  })
  loadedServerAutosave.value = null
  serverAutosaveVersion.value = 0
  serverDraftSavedAt.value = ''
  serverDraftSaveState.value = 'idle'
  serverDraftDirty = false
}

function writeLocalDraft(snapshot: ArticleDraftSnapshot) {
  try {
    localStorage.setItem(localDraftKey(), JSON.stringify(snapshot))
    return true
  } catch {
    return false
  }
}

function readLocalDraft(): ArticleDraftSnapshot | null {
  try {
    const raw = localStorage.getItem(localDraftKey())
    if (!raw) return null
    const value = JSON.parse(raw) as {
      schemaVersion?: unknown
      projectId?: unknown
      articleId?: unknown
      baseRevisionId?: unknown
      savedAt?: unknown
      fields?: unknown
    }
    if (
      value.schemaVersion === 1
      && value.projectId === projectID.value
      && value.articleId === articleID.value
      && typeof value.baseRevisionId === 'string'
      && typeof value.savedAt === 'string'
      && isLegacyArticleDraftFields(value.fields)
    ) {
      return {
        schemaVersion: 3,
        projectId: value.projectId,
        articleId: value.articleId,
        baseRevisionId: value.baseRevisionId,
        savedAt: value.savedAt,
        fields: {
          ...value.fields,
          contributors: revisionForm.contributors.map(contributor => ({ ...contributor })),
          attributionEdited: false,
          bodyDocument: articleBodyDocumentFromHTML(value.fields.html, value.fields.title)
        }
      }
    }
    if (
      value.schemaVersion === 2
      && value.projectId === projectID.value
      && value.articleId === articleID.value
      && typeof value.baseRevisionId === 'string'
      && typeof value.savedAt === 'string'
      && isArticleDraftFieldsV2(value.fields)
    ) {
      return {
        schemaVersion: 3,
        projectId: value.projectId,
        articleId: value.articleId,
        baseRevisionId: value.baseRevisionId,
        savedAt: value.savedAt,
        fields: {
          ...value.fields,
          bodyDocument: articleBodyDocumentFromHTML(value.fields.html, value.fields.title)
        }
      }
    }
    if (
      value.schemaVersion !== 3
      || value.projectId !== projectID.value
      || value.articleId !== articleID.value
      || typeof value.baseRevisionId !== 'string'
      || typeof value.savedAt !== 'string'
      || !isArticleDraftFields(value.fields)
    ) {
      removeLocalDraft()
      return null
    }
    return value as ArticleDraftSnapshot
  } catch {
    removeLocalDraft()
    return null
  }
}

function isLegacyArticleDraftFields(value: unknown): value is Omit<ArticleDraftFields, 'contributors' | 'attributionEdited' | 'bodyDocument'> {
  if (!value || typeof value !== 'object') return false
  const fields = value as Record<string, unknown>
  return [
    'title', 'primaryCategoryId', 'deck', 'excerpt', 'shortAnswer', 'seoTitle', 'seoDescription',
    'robots', 'openGraphTitle', 'openGraphDescription', 'openGraphImage', 'html'
  ].every(key => typeof fields[key] === 'string')
}

function isArticleDraftFields(value: unknown): value is ArticleDraftFields {
  return isArticleDraftFieldsV2(value)
    && isStructuredBodyDocument((value as Record<string, unknown>).bodyDocument)
}

function isArticleDraftFieldsV2(value: unknown): value is Omit<ArticleDraftFields, 'bodyDocument'> {
  if (!value || typeof value !== 'object') return false
  const fields = value as Record<string, unknown>
  const stringsAreValid = [
    'title', 'primaryCategoryId', 'deck', 'excerpt', 'shortAnswer', 'seoTitle', 'seoDescription',
    'robots', 'openGraphTitle', 'openGraphDescription', 'openGraphImage', 'html'
  ]
    .every(key => typeof fields[key] === 'string')
  return stringsAreValid
    && typeof fields.attributionEdited === 'boolean'
    && isContributorDraftValue(fields.contributors)
}

function isStructuredBodyDocument(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object'
    && (value as Record<string, unknown>).type === 'doc'
    && Array.isArray((value as Record<string, unknown>).content))
}

function hasMeaningfulStructuredHTML(value: string) {
  return Boolean(htmlToPlainText(value) || /<(?:img|hr|table)\b/i.test(value))
}

function isContributorDraftValue(value: unknown): value is RevisionContributorInput[] {
  return Array.isArray(value) && value.every((contributor) => {
    if (!contributor || typeof contributor !== 'object') return false
    const candidate = contributor as Record<string, unknown>
    return typeof candidate.authorId === 'string'
      && isContributorRole(candidate.role)
      && typeof candidate.position === 'number'
  })
}

function removeLocalDraft() {
  if (import.meta.client) localStorage.removeItem(localDraftKey())
}

function isCurrentPublication(revision: AdminRevisionSummary) {
  return revision.published
}

function assignmentDueAtForAPI() {
  if (!assignmentForm.dueAt) return undefined
  const date = new Date(assignmentForm.dueAt)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

function assignmentTypeAllowedForRole(assignmentType: string, role: string) {
  if (assignmentType === 'editor') {
    return ['project_owner', 'project_admin', 'editor'].includes(role)
  }
  if (assignmentType === 'reviewer' || assignmentType === 'sme') {
    return ['project_owner', 'project_admin', 'editor', 'reviewer'].includes(role)
  }
  return false
}

function canCompleteAssignment(assignment: ReviewAssignment) {
  return projectIsActive.value && (canManageAssignments.value || assignment.assignedTo === currentUser.value?.id)
}

function invalidateComparison() {
  comparisonRequestVersion += 1
  comparisonPending.value = false
  comparisonBefore.value = null
  comparisonAfter.value = null
}

function comparisonField(key: string, label: string, before: string, after: string, monospace = false, diffable = false): ComparisonField {
  const changed = before !== after
  return {
    key,
    label,
    before,
    after,
    changed,
    monospace,
    diffLines: changed && diffable ? buildLineDiff(before, after) : undefined
  }
}

function prettyJSON(value: unknown) {
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function buildLineDiff(before: string, after: string): ComparisonDiffLine[] {
  const beforeLines = normalizedDiffLines(before)
  const afterLines = normalizedDiffLines(after)
  let sharedPrefixLength = 0
  while (
    sharedPrefixLength < beforeLines.length
    && sharedPrefixLength < afterLines.length
    && beforeLines[sharedPrefixLength] === afterLines[sharedPrefixLength]
  ) {
    sharedPrefixLength += 1
  }

  let sharedSuffixLength = 0
  while (
    sharedSuffixLength < beforeLines.length - sharedPrefixLength
    && sharedSuffixLength < afterLines.length - sharedPrefixLength
    && beforeLines[beforeLines.length - sharedSuffixLength - 1] === afterLines[afterLines.length - sharedSuffixLength - 1]
  ) {
    sharedSuffixLength += 1
  }
  if (
    sharedPrefixLength === beforeLines.length
    && sharedPrefixLength === afterLines.length
    && before !== after
  ) {
    return [
      { kind: 'removed', text: 'The earlier value uses different line-ending characters.' },
      { kind: 'added', text: 'The later value uses different line-ending characters.' }
    ]
  }

  const beforeMiddle = beforeLines.slice(sharedPrefixLength, beforeLines.length - sharedSuffixLength)
  const afterMiddle = afterLines.slice(sharedPrefixLength, afterLines.length - sharedSuffixLength)
  const linesPerBlock = Math.max(1, Math.ceil(Math.max(beforeMiddle.length, afterMiddle.length) / 240))
  const beforeBlocks = groupDiffLines(beforeMiddle, linesPerBlock)
  const afterBlocks = groupDiffLines(afterMiddle, linesPerBlock)
  const table = Array.from(
    { length: beforeBlocks.length + 1 },
    () => new Uint16Array(afterBlocks.length + 1)
  )

  for (let beforeIndex = beforeBlocks.length - 1; beforeIndex >= 0; beforeIndex -= 1) {
    for (let afterIndex = afterBlocks.length - 1; afterIndex >= 0; afterIndex -= 1) {
      table[beforeIndex]![afterIndex] = beforeBlocks[beforeIndex] === afterBlocks[afterIndex]
        ? (table[beforeIndex + 1]![afterIndex + 1] || 0) + 1
        : Math.max(table[beforeIndex + 1]![afterIndex] || 0, table[beforeIndex]![afterIndex + 1] || 0)
    }
  }

  const diff: ComparisonDiffLine[] = []
  let beforeIndex = 0
  let afterIndex = 0
  while (beforeIndex < beforeBlocks.length && afterIndex < afterBlocks.length) {
    if (beforeBlocks[beforeIndex] === afterBlocks[afterIndex]) {
      diff.push({ kind: 'equal', text: beforeBlocks[beforeIndex] || '' })
      beforeIndex += 1
      afterIndex += 1
    } else if ((table[beforeIndex + 1]![afterIndex] || 0) >= (table[beforeIndex]![afterIndex + 1] || 0)) {
      diff.push({ kind: 'removed', text: beforeBlocks[beforeIndex] || '' })
      beforeIndex += 1
    } else {
      diff.push({ kind: 'added', text: afterBlocks[afterIndex] || '' })
      afterIndex += 1
    }
  }
  while (beforeIndex < beforeBlocks.length) {
    diff.push({ kind: 'removed', text: beforeBlocks[beforeIndex] || '' })
    beforeIndex += 1
  }
  while (afterIndex < afterBlocks.length) {
    diff.push({ kind: 'added', text: afterBlocks[afterIndex] || '' })
    afterIndex += 1
  }

  const result: ComparisonDiffLine[] = []
  const contextLines = 2
  const visiblePrefixStart = Math.max(0, sharedPrefixLength - contextLines)
  if (visiblePrefixStart > 0) {
    result.push({ kind: 'omitted', text: `${visiblePrefixStart} unchanged line${visiblePrefixStart === 1 ? '' : 's'} omitted` })
  }
  for (let index = visiblePrefixStart; index < sharedPrefixLength; index += 1) {
    result.push({ kind: 'equal', text: beforeLines[index] || '' })
  }
  result.push(...compactDiffContext(diff))

  const visibleSuffixLength = Math.min(contextLines, sharedSuffixLength)
  const suffixStart = beforeLines.length - sharedSuffixLength
  for (let index = 0; index < visibleSuffixLength; index += 1) {
    result.push({ kind: 'equal', text: beforeLines[suffixStart + index] || '' })
  }
  const omittedSuffixLength = sharedSuffixLength - visibleSuffixLength
  if (omittedSuffixLength > 0) {
    result.push({ kind: 'omitted', text: `${omittedSuffixLength} unchanged line${omittedSuffixLength === 1 ? '' : 's'} omitted` })
  }
  return result
}

function normalizedDiffLines(value: string) {
  return value.replaceAll('\r\n', '\n').replaceAll('\r', '\n').split('\n')
}

function groupDiffLines(lines: string[], linesPerBlock: number) {
  if (linesPerBlock === 1) return lines
  const blocks: string[] = []
  for (let index = 0; index < lines.length; index += linesPerBlock) {
    blocks.push(lines.slice(index, index + linesPerBlock).join('\n'))
  }
  return blocks
}

function compactDiffContext(lines: ComparisonDiffLine[]) {
  const context = 2
  const keep = new Set<number>()
  lines.forEach((line, index) => {
    if (line.kind === 'equal') return
    for (let candidate = Math.max(0, index - context); candidate <= Math.min(lines.length - 1, index + context); candidate += 1) {
      keep.add(candidate)
    }
  })

  const compacted: ComparisonDiffLine[] = []
  let omitted = 0
  lines.forEach((line, index) => {
    if (!keep.has(index)) {
      omitted += 1
      return
    }
    if (omitted > 0) {
      compacted.push({ kind: 'omitted', text: `${omitted} unchanged diff section${omitted === 1 ? '' : 's'} omitted` })
      omitted = 0
    }
    compacted.push(line)
  })
  if (omitted > 0) {
    compacted.push({ kind: 'omitted', text: `${omitted} unchanged diff section${omitted === 1 ? '' : 's'} omitted` })
  }
  return compacted
}

function diffLineClass(kind: ComparisonDiffLine['kind']) {
  switch (kind) {
    case 'added':
      return 'bg-[#edf9f1] text-[#165a4a] dark:bg-[#13261e] dark:text-[#aee4d0]'
    case 'removed':
      return 'bg-[#fff4f2] text-[#9b2d23] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]'
    case 'omitted':
      return 'bg-[#f2f5f3] italic text-[#667169] dark:bg-[#202522] dark:text-[#aeb8b0]'
    default:
      return 'text-[#4f5b54] dark:text-[#c5cec8]'
  }
}

function diffLineMarker(kind: ComparisonDiffLine['kind']) {
  if (kind === 'added') return '+'
  if (kind === 'removed') return '−'
  if (kind === 'omitted') return '⋯'
  return ' '
}

function diffLineLabel(kind: ComparisonDiffLine['kind']) {
  if (kind === 'added') return 'Added'
  if (kind === 'removed') return 'Removed'
  if (kind === 'omitted') return 'Unchanged context omitted'
  return 'Unchanged'
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
  return [...values].sort((left, right) => categoryPathLabel(left).localeCompare(categoryPathLabel(right)))
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

function assignmentStatusClass(status: string) {
  switch (status) {
    case 'completed':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'cancelled':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function assignmentTypeClass(type: string) {
  switch (type) {
    case 'editor':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'sme':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
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

function apiErrorStatus(error: unknown) {
  if (typeof error !== 'object' || error === null) return 0
  const value = error as { response?: { status?: number }, status?: number, statusCode?: number }
  return value.response?.status || value.status || value.statusCode || 0
}
</script>
