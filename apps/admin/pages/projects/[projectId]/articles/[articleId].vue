<template>
  <section class="article-detail min-h-screen">
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
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]" role="status" aria-live="polite">
          {{ successMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading article
        </div>

        <div v-else-if="article" class="article-workspace">
          <header class="article-workspace__hero">
            <div class="article-workspace__title">
              <p>{{ labelize(article.articleType) }}</p>
              <h1>{{ article.title }}</h1>
              <span>/{{ article.slug }}</span>
            </div>
            <div class="article-workspace__status">
              <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="publicationClass(article.publicationState)">{{ labelize(article.publicationState) }}</span>
              <button
                v-if="canPublishArticles && workspaceTab !== 'publish'"
                class="article-workspace__publish-shortcut"
                type="button"
                @click="workspaceTab = 'publish'"
              >
                <UploadCloud :size="15" />
                {{ article.publicationState === 'published' ? 'Publish changes' : 'Publish' }}
              </button>
            </div>
          </header>

          <nav class="article-workspace__tabs" aria-label="Article workspace">
            <button v-if="canWriteArticles" type="button" :class="{ 'is-active': workspaceTab === 'write' }" :aria-current="workspaceTab === 'write' ? 'page' : undefined" @click="workspaceTab = 'write'">
              <FilePenLine :size="17" /><span>Write</span>
            </button>
            <button type="button" :class="{ 'is-active': workspaceTab === 'overview' }" :aria-current="workspaceTab === 'overview' ? 'page' : undefined" @click="workspaceTab = 'overview'">
              <Hash :size="17" /><span>Overview</span>
            </button>
            <button v-if="canPublishArticles" type="button" :class="{ 'is-active': workspaceTab === 'publish' }" :aria-current="workspaceTab === 'publish' ? 'page' : undefined" @click="workspaceTab = 'publish'">
              <UploadCloud :size="17" /><span>Publish</span>
            </button>
          </nav>

          <div v-show="workspaceTab === 'overview'" class="article-workspace__panel-stack">
            <article v-show="workspaceTab === 'overview'" class="article-overview__summary rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ labelize(article.articleType) }}</p>
                  <h2 class="mt-1 truncate text-xl font-semibold tracking-normal">{{ article.title }}</h2>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ article.slug }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
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
          </div>

          <div v-show="workspaceTab === 'write' || workspaceTab === 'overview' || workspaceTab === 'publish'" class="article-workspace__panel-stack" :class="{ 'article-publish-grid': workspaceTab === 'publish' }">
            <form v-if="canWriteArticles" v-show="workspaceTab === 'write'" class="article-compose space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="saveArticle">
              <div class="flex items-start gap-3">
                <FilePenLine class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div class="min-w-0 flex-1">
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Article</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Edit content</h2>
                  <p v-if="draftStatusText" class="mt-1 text-xs" :class="draftStatusClass" aria-live="polite">
                    {{ draftStatusText }}
                  </p>
                </div>
              </div>

              <div
                v-if="staleDraft"
                class="rounded-md border border-[#e1bd70] bg-[#fff8e7] p-3 text-sm text-[#6b4905] dark:border-[#665223] dark:bg-[#2b2415] dark:text-[#f5d992]"
              >
                <p class="font-medium">{{ staleDraft.reason === 'version-conflict' ? 'Another tab saved different changes.' : 'This draft was saved before the article was last updated.' }}</p>
                <p class="mt-1 text-xs">
                  {{ staleDraft.reason === 'version-conflict'
                    ? `This tab's browser backup from ${formatDate(staleDraft.snapshot.savedAt)} is still available.`
                    : `The article changed after this ${staleDraft.source === 'server' ? 'server' : 'browser'} draft was saved ${formatDate(staleDraft.snapshot.savedAt)}. Restore it to inspect the changes, or discard it.` }}
                </p>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button class="rounded-md border border-current px-3 py-1.5 text-xs font-medium" type="button" @click="restoreStaleDraft">Restore draft</button>
                  <button class="rounded-md px-3 py-1.5 text-xs font-medium underline" type="button" @click="discardLocalDraft">Discard</button>
                </div>
              </div>

              <div
                v-else-if="serverDraftSaveState === 'conflict'"
                class="rounded-md border border-[#e1bd70] bg-[#fff8e7] p-3 text-sm text-[#6b4905] dark:border-[#665223] dark:bg-[#2b2415] dark:text-[#f5d992]"
              >
                <p class="font-medium">Autosave paused because another tab saved newer work.</p>
                <p class="mt-1 text-xs">Reload the latest saved draft before continuing. This tab's browser backup will remain available.</p>
                <button class="mt-3 rounded-md border border-current px-3 py-1.5 text-xs font-medium disabled:opacity-60" type="button" :disabled="reloadingServerDraft" @click="reloadServerDraft">
                  {{ reloadingServerDraft ? 'Reloading…' : 'Reload saved draft' }}
                </button>
              </div>

              <p
                v-else-if="draftSaveState === 'restored'"
                class="rounded-md border border-[#b9d5c8] bg-[#eef8f3] px-3 py-2 text-xs text-[#165a4a] dark:border-[#315648] dark:bg-[#14251f] dark:text-[#aee4d0]"
              >
                Your saved working draft was restored.
              </p>

              <label class="article-compose__title block space-y-2">
                <span class="text-sm font-medium">Title</span>
                <input v-model.trim="articleForm.title" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Primary category</span>
                <select v-model="articleForm.primaryCategoryId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option value="">Keep current category</option>
                  <option v-for="category in categories" :key="category.id" :value="category.id">{{ categoryPathLabel(category) }}</option>
                </select>
              </label>
              <ArticleContributorsEditor
                :model-value="articleForm.contributors"
                :authors="authors"
                @update:model-value="updateArticleContributors"
              />
              <ArticleStructuredEditor
                v-model:html="articleForm.html"
                v-model:body-document="articleBodyDocument"
                label="Article body"
                :media-assets="mediaAssets"
                :sources="sources"
              />
              <div class="article-compose__summaries">
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Deck <small>Optional subtitle</small></span>
                  <textarea v-model.trim="articleForm.deck" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Excerpt <small>Article summary</small></span>
                  <textarea v-model.trim="articleForm.excerpt" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Short answer <small>Direct answer for search</small></span>
                  <textarea v-model.trim="articleForm.shortAnswer" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
              </div>
              <details class="article-compose__advanced">
                <summary>
                  <span><strong>SEO and social preview</strong><small>Search metadata, robots, and Open Graph</small></span>
                  <ChevronDown :size="17" />
                </summary>
                <div class="grid gap-3 p-3 sm:grid-cols-2">
                <label class="block space-y-2">
                  <span class="text-sm font-medium">SEO title</span>
                  <input v-model.trim="articleForm.seoTitle" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Defaults to article title" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Robots</span>
                  <select v-model="articleForm.robots" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                    <option value="index,follow">Index, follow</option>
                    <option value="index,nofollow">Index, nofollow</option>
                    <option value="noindex,follow">Noindex, follow</option>
                    <option value="noindex,nofollow">Noindex, nofollow</option>
                  </select>
                </label>
                <label class="block space-y-2 sm:col-span-2">
                  <span class="text-sm font-medium">Meta description</span>
                  <textarea v-model.trim="articleForm.seoDescription" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Defaults to excerpt" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Open Graph title</span>
                  <input v-model.trim="articleForm.openGraphTitle" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Open Graph image URL</span>
                  <input v-model.trim="articleForm.openGraphImage" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="https://…" />
                </label>
                <label class="block space-y-2 sm:col-span-2">
                  <span class="text-sm font-medium">Open Graph description</span>
                  <textarea v-model.trim="articleForm.openGraphDescription" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
                </label>
                </div>
              </details>
              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="savingArticle || !canSaveArticle"
              >
                <LoaderCircle v-if="savingArticle" class="h-4 w-4 animate-spin" />
                <CheckCircle2 v-else class="h-4 w-4" />
                Save changes
              </button>
            </form>

            <form
              v-if="projectIsActive && copyDestinations.length"
              v-show="workspaceTab === 'overview'"
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
                The canonical URL is resolved by the server from the original article. It cannot be redirected to another URL.
              </p>
              <p class="rounded-md bg-[#f2f5f3] px-3 py-2 text-xs text-[#5d6a61] dark:bg-[#171b18] dark:text-[#aeb8b0]">
                The latest saved content becomes a new unpublished draft in the destination project. Choose its taxonomy before copying.
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

            <form v-if="canPublishArticles" v-show="workspaceTab === 'publish'" class="article-publish__primary rounded-lg border border-[#b9dcc9] bg-white p-5 shadow-sm dark:border-[#2d644a] dark:bg-[#202522]" @submit.prevent="publishArticle">
              <div class="article-publish__lead">
                <div class="article-publish__icon"><UploadCloud :size="22" /></div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-[#165a4a] dark:text-[#aee4d0]">
                    {{ article.publicationState === 'published' ? 'Changes ready to publish' : 'Ready to publish' }}
                  </p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">
                    {{ article.publicationState === 'published' ? 'Publish the latest changes' : 'Publish this article' }}
                  </h2>
                  <p class="mt-2 max-w-2xl text-sm text-[#5d6a61] dark:text-[#aeb8b0]">
                    Your latest content will be saved, then go live immediately at <strong>/{{ publicationForm.slug }}</strong>.
                  </p>
                </div>
              </div>

              <details class="article-publish__details">
                <summary>
                  <span><strong>URL settings</strong><small>Slug and optional canonical URL</small></span>
                  <ChevronDown :size="17" />
                </summary>
                <div class="grid gap-4 p-4 md:grid-cols-2">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">Slug</span>
                    <input v-model.trim="publicationForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
                  </label>
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">Canonical URL <small class="font-normal text-[#667169] dark:text-[#aeb8b0]">Optional</small></span>
                    <input v-model.trim="publicationForm.canonicalUrl" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="url" placeholder="Generated automatically" />
                  </label>
                </div>
              </details>

              <div class="mt-5 flex flex-wrap items-center gap-3">
                <button
                  class="inline-flex h-11 min-w-44 items-center justify-center gap-2 rounded-md bg-[#165a4a] px-5 text-sm font-semibold text-white shadow-sm hover:bg-[#10463a] disabled:opacity-60"
                  type="submit"
                  :disabled="Boolean(actionPending) || savingArticle || !publicationForm.slug"
                >
                  <LoaderCircle v-if="actionPending === 'publish'" class="h-4 w-4 animate-spin" />
                  <UploadCloud v-else class="h-4 w-4" />
                  {{ article.publicationState === 'published' ? 'Publish latest changes' : 'Publish now' }}
                </button>
                <button
                  v-if="article.publicationState !== 'unpublished'"
                  class="inline-flex h-11 items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-4 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                  type="button"
                  :disabled="Boolean(actionPending)"
                  @click="unpublishArticle"
                >
                  <LoaderCircle v-if="actionPending === 'unpublish'" class="h-4 w-4 animate-spin" />
                  <XCircle v-else class="h-4 w-4" />
                  {{ article.publicationState === 'scheduled' ? 'Cancel schedule' : 'Unpublish' }}
                </button>
              </div>
            </form>

            <details v-if="canPublishArticles && article.publicationState !== 'published'" v-show="workspaceTab === 'publish'" class="article-publish__advanced article-publish__schedule">
              <summary>
                <span>
                  <strong>Schedule for later</strong>
                  <small v-if="article.scheduledForUtc">Currently scheduled for {{ formatDate(article.scheduledForUtc) }}</small>
                  <small v-else>Choose a date and time instead of publishing now</small>
                </span>
                <ChevronDown :size="18" />
              </summary>
              <form class="article-publish__schedule-form" @submit.prevent="scheduleArticle">
                <label>
                  <span class="sr-only">Publish date and time</span>
                  <input
                    v-model="scheduleDraft"
                    class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
                    type="datetime-local"
                    :min="minimumScheduleDate"
                    required
                  />
                </label>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
                  type="submit"
                  :disabled="Boolean(actionPending) || savingArticle || !scheduleDraft"
                >
                  <LoaderCircle v-if="actionPending === 'schedule'" class="h-4 w-4 animate-spin" />
                  <CalendarClock v-else class="h-4 w-4" />
                  {{ article.scheduledForUtc ? 'Update schedule' : 'Schedule' }}
                </button>
              </form>
            </details>

            <details v-if="canPublishArticles" v-show="workspaceTab === 'publish'" class="article-publish__advanced">
              <summary>
                <span>
                  <strong>Article management</strong>
                  <small>Archive this article</small>
                </span>
                <ChevronDown :size="18" />
              </summary>

              <div class="article-publish__advanced-content">
                <section class="article-publish__option article-publish__controls">
                  <div class="flex items-start gap-3">
                    <Trash2 class="mt-1 h-4 w-4 text-[#9b2d23] dark:text-[#ffc4bd]" />
                    <div>
                      <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Article management</p>
                      <h2 class="mt-1 text-base font-semibold tracking-normal">Archive article</h2>
                    </div>
                  </div>
                  <p class="text-sm text-[#5f6a63] dark:text-[#b8c2bb]">
                    Hides the article from the list while keeping its saved content. Published articles are unpublished first.
                  </p>
                  <div class="flex flex-wrap gap-2">
                    <button
                      class="inline-flex h-10 flex-1 items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-4 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                      type="button"
                      :disabled="Boolean(actionPending) || !canArchiveArticle"
                      @click="archiveArticle"
                    >
                      <LoaderCircle v-if="actionPending === 'archive'" class="h-4 w-4 animate-spin" />
                      <Trash2 v-else class="h-4 w-4" />
                      Archive article
                    </button>
                  </div>
                </section>
              </div>
            </details>
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
  ChevronDown,
  CopyPlus,
  FilePenLine,
  Hash,
  LoaderCircle,
  LogOut,
  RefreshCw,
  Trash2,
  UploadCloud,
  XCircle
} from 'lucide-vue-next'
import type { AdminAuthor, AdminMediaAsset, AdminSource, ArticleContributorInput } from '~/composables/useAdminApi'
import { articleBodyDocumentFromHTML, hasValidArticleContributors, htmlToPlainText } from '~/composables/useAdminApi'

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
}

type LatestContentVersion = {
  id: string
}

type ArticleSEO = {
  title?: string
  description?: string
  robots?: string
  openGraphTitle?: string
  openGraphDescription?: string
  openGraphImage?: string
}

type ArticleDraftFields = {
  title: string
  primaryCategoryId: string
  contributors: ArticleContributorInput[]
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
  deck?: string
  excerpt?: string
  shortAnswer?: string
  primaryCategoryId?: string
  contributors?: ArticleContributorInput[]
  bodyDocument?: unknown
  html?: string
  seo?: ArticleSEO
  publicationState: string
  canonicalPolicy: string
  scheduledForUtc?: string
  publishedAt?: string
  canonicalUrl?: string
  latestRevision?: LatestContentVersion
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
const projects = ref<AdminProject[]>([])
const article = ref<AdminArticle | null>(null)
const workspaceTab = ref<'write' | 'overview' | 'publish'>('write')
const categories = ref<TaxonomyTerm[]>([])
const authors = ref<AdminAuthor[]>([])
const mediaAssets = ref<AdminMediaAsset[]>([])
const sources = ref<AdminSource[]>([])
const copyDestinationCategories = ref<TaxonomyTerm[]>([])
const copyDestinationAuthors = ref<AdminAuthor[]>([])
const copyContributorMappings = reactive<Record<string, string>>({})
const pending = ref(true)
const savingArticle = ref(false)
const copyingArticle = ref(false)
const loadingCopyCategories = ref(false)
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
const contentDirty = ref(false)
const articleBodyDocument = ref<unknown>({ type: 'doc', schemaVersion: 'tiptap-v1', content: [] })
const baseContentVersionID = ref('')
const scheduleDraft = ref('')
const minimumScheduleDate = toLocalInputValue(new Date(Date.now() + 60_000))
const attributionEdited = ref(false)
let copyCategoryRequestVersion = 0
let copyContextRequestVersion = 0
let draftPersistenceEnabled = false
let draftDirty = false
let draftSaveTimer: ReturnType<typeof setTimeout> | undefined
let serverDraftDirty = false
let serverSaveInFlight = false
let serverSaveTimer: ReturnType<typeof setTimeout> | undefined
let serverSaveGeneration = 0

const articleForm = reactive({
  title: '',
  primaryCategoryId: '',
  contributors: [] as ArticleContributorInput[],
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

const copyForm = reactive({
  destinationProjectId: '',
  primaryCategoryId: '',
  slug: '',
  canonicalDecision: 'material_adaptation'
})

const projectIsActive = computed(() => project.value?.status === 'active')
const canWriteArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'writer'].includes(project.value?.role || ''))
const canPublishArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
watch(canWriteArticles, (canWrite) => {
  if (!canWrite && workspaceTab.value === 'write') workspaceTab.value = 'overview'
})
const canSaveArticle = computed(() => canWriteArticles.value && Boolean(
  articleForm.title.trim()
  && hasValidArticleContributors(articleForm.contributors)
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
  && copyForm.primaryCategoryId
  && copyForm.slug.trim()
  && ['canonical_original', 'material_adaptation'].includes(copyForm.canonicalDecision)
  && copySourceContributors.value.every(contributor => copyContributorMappings[contributor.authorId])
))
const copySourceContributors = computed(() => sourceContributorsFromArticle(article.value))
const canArchiveArticle = computed(() => canPublishArticles.value)

watch(
  () => copyForm.destinationProjectId,
  destinationProjectId => loadCopyContext(destinationProjectId)
)

watch(
  () => ({ ...articleForm, bodyDocument: articleBodyDocument.value }),
  () => {
    if (!draftPersistenceEnabled || !import.meta.client) return
    contentDirty.value = true
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
    const [projectResponse, projectListResponse, categoryResponse, authorResponse, articleResponse, autosaveResponse, mediaResponse, sourceResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      fetchAllCopyProjects(),
      fetchAllCategories(projectID.value),
      $fetch<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID.value}/authors`, { credentials: 'include' }),
      $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}`, { credentials: 'include' }),
      fetchArticleAutosave(),
      $fetch<APIListEnvelope<AdminMediaAsset>>(`/api/v1/projects/${projectID.value}/media`, { credentials: 'include' }),
      $fetch<APIListEnvelope<AdminSource>>(`/api/v1/projects/${projectID.value}/sources`, { credentials: 'include', query: { limit: 100 } })
    ])
    project.value = projectResponse.data
    if (!canWriteArticles.value && workspaceTab.value === 'write') workspaceTab.value = 'overview'
    projects.value = projectListResponse
    categories.value = sortCategories(categoryResponse)
    authors.value = apiListData(authorResponse).sort((left, right) => left.displayName.localeCompare(right.displayName))
    mediaAssets.value = apiListData(mediaResponse)
    sources.value = apiListData(sourceResponse)
    loadedServerAutosave.value = autosaveResponse
    serverAutosaveVersion.value = autosaveResponse?.version || 0
    setArticle(articleResponse.data)
    handleLoadedArticle(articleResponse.data)
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

async function saveArticle() {
  if (!article.value || savingArticle.value) return false
  savingArticle.value = true
  clearMessages()
  serverSaveGeneration += 1
  if (serverSaveTimer) {
    clearTimeout(serverSaveTimer)
    serverSaveTimer = undefined
  }
  try {
    const pendingPublicationSettings = { ...publicationForm }
    const csrfToken = await getCSRFToken()
    const html = effectiveDraftHTML()
    const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${article.value.id}`, {
      method: 'PUT',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        baseRevisionId: baseContentVersionID.value || latestContentVersionID(),
        title: articleForm.title,
        primaryCategoryId: articleForm.primaryCategoryId,
        contributors: articleForm.contributors,
        deck: articleForm.deck,
        excerpt: articleForm.excerpt,
        shortAnswer: articleForm.shortAnswer,
        bodyDocument: draftBodyDocument(),
        html,
        seo: {
          title: articleForm.seoTitle,
          description: articleForm.seoDescription,
          robots: articleForm.robots,
          openGraphTitle: articleForm.openGraphTitle,
          openGraphDescription: articleForm.openGraphDescription,
          openGraphImage: articleForm.openGraphImage
        }
      }
    })
    draftPersistenceEnabled = false
    if (serverSaveTimer) {
      clearTimeout(serverSaveTimer)
      serverSaveTimer = undefined
    }
    removeLocalDraft()
    successMessage.value = 'Changes saved.'
    setArticle(response.data)
    Object.assign(publicationForm, pendingPublicationSettings)
    setArticleForm(response.data)
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
    contentDirty.value = false
    draftPersistenceEnabled = true
    return true
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not save changes.')
    serverDraftDirty = true
    if (serverDraftSaveState.value !== 'conflict') {
      serverDraftSaveState.value = 'saving'
      queueServerAutosave(100)
    }
    return false
  } finally {
    savingArticle.value = false
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

async function loadCopyContext(destinationProjectId: string) {
  const requestVersion = ++copyContextRequestVersion
  copyDestinationAuthors.value = []
  for (const key of Object.keys(copyContributorMappings)) delete copyContributorMappings[key]
  await loadCopyDestinationCategories(destinationProjectId)
  if (!destinationProjectId || requestVersion !== copyContextRequestVersion) return
  try {
    const authorResponse = await fetchAllAuthors(destinationProjectId)
    if (requestVersion !== copyContextRequestVersion) return
    copyDestinationAuthors.value = authorResponse
      .filter(author => author.status === 'active')
      .sort((left, right) => left.displayName.localeCompare(right.displayName))
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

function sourceContributorsFromArticle(value: AdminArticle | null) {
  const values = new Map<string, { authorId: string, name: string, roles: string[] }>()
  for (const contributor of value?.contributors || []) {
    const role = String(contributor.role || 'contributor').replaceAll('_', ' ')
    const existing = values.get(contributor.authorId)
    if (existing) {
      if (!existing.roles.includes(role)) existing.roles.push(role)
      continue
    }
    const author = authors.value.find(candidate => candidate.id === contributor.authorId)
    values.set(contributor.authorId, {
      authorId: contributor.authorId,
      name: author?.displayName || contributor.authorId,
      roles: [role]
    })
  }
  return [...values.values()]
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

async function publishArticle() {
  if (!validatePublicationSettings()) return
  if (!await saveBeforePublication()) return
  await mutateArticle('publish', async (csrfToken) => {
    const body = publicationBody()
    const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/publish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      ...(Object.keys(body).length ? { body } : {})
    })
    successMessage.value = 'Article published.'
    return response.data
  })
}

async function scheduleArticle() {
  if (!scheduleDraft.value) return
  if (!validatePublicationSettings()) return
  if (!await saveBeforePublication()) return
  const scheduledAt = new Date(scheduleDraft.value)
  if (Number.isNaN(scheduledAt.getTime()) || scheduledAt.getTime() <= Date.now()) {
    errorMessage.value = 'Choose a valid future publication time.'
    return
  }
  await mutateArticle('schedule', async (csrfToken) => {
    const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/schedule`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        ...publicationBody(),
        scheduledForUtc: scheduledAt.toISOString()
      }
    })
    successMessage.value = 'Article scheduled.'
    return response.data
  })
}

function validatePublicationSettings() {
  if (!publicationForm.slug.trim()) {
    errorMessage.value = 'Enter a URL slug before publishing.'
    return false
  }
  const canonical = publicationForm.canonicalUrl.trim()
  if (canonical) {
    try {
      const parsed = new URL(canonical)
      if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error('unsupported protocol')
    } catch {
      errorMessage.value = 'Enter a valid HTTP or HTTPS canonical URL.'
      return false
    }
  }
  return true
}

async function saveBeforePublication() {
  if (!contentDirty.value) return true
  if (!canWriteArticles.value || !canSaveArticle.value) {
    workspaceTab.value = 'write'
    errorMessage.value = 'Complete the required article fields before publishing.'
    return false
  }
  const saved = await saveArticle()
  if (!saved) workspaceTab.value = 'write'
  return saved
}

async function unpublishArticle() {
  const cancellingSchedule = article.value?.publicationState === 'scheduled'
  if (!window.confirm(cancellingSchedule ? 'Cancel this scheduled publication?' : 'Unpublish this article?')) return
  await mutateArticle('unpublish', async (csrfToken) => {
    const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${articleID.value}/unpublish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
    successMessage.value = cancellingSchedule ? 'Schedule cancelled.' : 'Article unpublished.'
    return response.data
  })
}

async function archiveArticle() {
  if (!article.value || !canArchiveArticle.value) return
  const message = article.value.publicationState === 'published'
    ? `Archive "${article.value.title}"? This will unpublish it from the content API and hide it from the admin article list.`
    : `Archive "${article.value.title}"? This will hide it from the admin article list while keeping its saved content.`
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

async function mutateArticle(action: string, operation: (csrfToken: string) => Promise<AdminArticle>) {
  actionPending.value = action
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    setArticle(await operation(csrfToken))
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, `Could not ${labelize(action)} article.`)
  } finally {
    actionPending.value = ''
  }
}

function setArticle(value: AdminArticle) {
  article.value = value
  publicationForm.slug = value.slug
  publicationForm.canonicalUrl = value.canonicalUrl || ''
  copyForm.slug = copyForm.slug || `${value.slug}-copy`
  if (!scheduleDraft.value) {
    const scheduledAt = value.scheduledForUtc ? parseBackendUTC(value.scheduledForUtc) : new Date(Date.now() + 15 * 60 * 1000)
    scheduleDraft.value = toLocalInputValue(scheduledAt)
  }
}

function setArticleForm(value: AdminArticle) {
  const seo = value.seo || {}
  articleForm.title = value.title
  articleForm.primaryCategoryId = value.primaryCategoryId || ''
  articleForm.contributors = (value.contributors || []).map(contributor => ({ ...contributor }))
  attributionEdited.value = false
  articleForm.deck = value.deck || ''
  articleForm.excerpt = value.excerpt || ''
  articleForm.shortAnswer = value.shortAnswer || ''
  articleForm.seoTitle = seo.title || ''
  articleForm.seoDescription = seo.description || ''
  articleForm.robots = seo.robots || 'index,follow'
  articleForm.openGraphTitle = seo.openGraphTitle || ''
  articleForm.openGraphDescription = seo.openGraphDescription || ''
  articleForm.openGraphImage = seo.openGraphImage || ''
  articleForm.html = value.html || `<p>${escapeHTML(value.title)}</p>`
  articleBodyDocument.value = isStructuredBodyDocument(value.bodyDocument)
    ? value.bodyDocument
    : articleBodyDocumentFromHTML(articleForm.html, articleForm.title)
  baseContentVersionID.value = value.latestRevision?.id || ''
  contentDirty.value = false
}

function handleLoadedArticle(value: AdminArticle) {
  const contentVersionID = value.latestRevision?.id || ''
  if (!draftPersistenceEnabled || !baseContentVersionID.value) {
    setArticleForm(value)
    return
  }
  if (contentVersionID === baseContentVersionID.value) return

  if (draftDirty) persistLocalDraft()
  const localDraft = readLocalDraft()
  if (serverSaveTimer) {
    clearTimeout(serverSaveTimer)
    serverSaveTimer = undefined
  }
  draftPersistenceEnabled = false
  setArticleForm(value)
  draftDirty = false
  serverDraftDirty = false
  draftSaveState.value = 'idle'
  draftSavedAt.value = ''
  serverDraftSaveState.value = 'idle'
  serverDraftSavedAt.value = ''
  const recoveries: ArticleDraftRecovery[] = []
  if (localDraft && localDraft.baseRevisionId !== contentVersionID) {
    recoveries.push({ snapshot: localDraft, source: 'browser', reason: 'stale-base' })
  }
  if (loadedServerAutosave.value && loadedServerAutosave.value.baseRevisionId !== contentVersionID) {
    recoveries.push(articleAutosaveRecovery(loadedServerAutosave.value))
  }
  staleDraft.value = newestDraftRecovery(recoveries)
  nextTick(() => {
    draftPersistenceEnabled = true
  })
}

function isContributorRole(value: unknown): value is ArticleContributorInput['role'] {
  return ['primary_author', 'co_author', 'editor', 'expert_reviewer', 'photographer', 'other'].includes(String(value))
}

function updateArticleContributors(value: ArticleContributorInput[]) {
  articleForm.contributors = value
  attributionEdited.value = true
}

function categoryPathLabel(category: TaxonomyTerm) {
  return [...(category.ancestors || []).map(ancestor => ancestor.name), category.name].join(' / ')
}

function publicationBody() {
  const body: Record<string, string> = {}
  const slug = publicationForm.slug.trim()
  const canonicalUrl = publicationForm.canonicalUrl.trim()
  if (slug && slug !== article.value?.slug) body.slug = slug
  if (canonicalUrl !== (article.value?.canonicalUrl || '')) body.canonicalUrl = canonicalUrl
  return body
}

function latestContentVersionID() {
  return article.value?.latestRevision?.id || ''
}

function localDraftKey() {
  return `seoblog:article-draft:${projectID.value}:${articleID.value}`
}

function draftFieldsSnapshot(): ArticleDraftFields {
  return {
    ...articleForm,
    contributors: articleForm.contributors.map(contributor => ({ ...contributor })),
    attributionEdited: attributionEdited.value,
    bodyDocument: draftBodyDocument()
  }
}

function draftSnapshot(savedAt = new Date().toISOString()): ArticleDraftSnapshot {
  return {
    schemaVersion: 3,
    projectId: projectID.value,
    articleId: articleID.value,
    baseRevisionId: baseContentVersionID.value || latestContentVersionID(),
    savedAt,
    fields: draftFieldsSnapshot()
  }
}

function effectiveDraftHTML() {
  return hasMeaningfulStructuredHTML(articleForm.html)
    ? articleForm.html.trim()
    : `<p>${escapeHTML(articleForm.title)}</p>`
}

function draftBodyDocument() {
  return hasMeaningfulStructuredHTML(articleForm.html) && isStructuredBodyDocument(articleBodyDocument.value)
    ? articleBodyDocument.value
    : articleBodyDocumentFromHTML(effectiveDraftHTML(), articleForm.title)
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
    errorMessage.value = 'This browser could not save the draft locally.'
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
  const baseRevisionId = baseContentVersionID.value || latestContentVersionID()
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

  const currentRecoveries = recoveries.filter(recovery => recovery.snapshot.baseRevisionId === latestContentVersionID())
  const staleRecoveries = recoveries.filter(recovery => recovery.snapshot.baseRevisionId !== latestContentVersionID())
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
      && browserRecovery.baseRevisionId !== latestContentVersionID()
    ) {
      staleDraft.value = { snapshot: browserRecovery, source: 'browser', reason: 'stale-base' }
    } else if (
      recovery.source === 'browser'
      && loadedServerAutosave.value
      && loadedServerAutosave.value.baseRevisionId !== latestContentVersionID()
    ) {
      staleDraft.value = articleAutosaveRecovery(loadedServerAutosave.value)
    } else {
      staleDraft.value = null
    }
    if (
      recovery.source === 'browser'
      && !staleDraft.value
      && loadedServerAutosave.value?.baseRevisionId === latestContentVersionID()
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
  Object.assign(articleForm, fields)
  attributionEdited.value = savedAttributionEdited
  articleBodyDocument.value = bodyDocument
  draftSavedAt.value = snapshot.savedAt
  contentDirty.value = true
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
    if (autosave.stale || recovery.snapshot.baseRevisionId !== latestContentVersionID()) {
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
          contributors: articleForm.contributors.map(contributor => ({ ...contributor })),
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

function isContributorDraftValue(value: unknown): value is ArticleContributorInput[] {
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

<style scoped>
.article-detail {
  min-height: 100%;
  color: var(--text);
}

.article-workspace {
  display: grid;
  min-width: 0;
  gap: 18px;
}

.article-workspace__hero {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: flex-start;
  gap: 18px;
  padding: 24px 26px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--primary-soft) 70%, transparent), transparent 42%),
    var(--surface);
  box-shadow: var(--shadow-sm);
}

.article-workspace__hero::before {
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: linear-gradient(90deg, var(--primary), #3162a3, #a16207);
  content: "";
}

.article-workspace__title {
  position: relative;
  min-width: 0;
}

.article-workspace__title p {
  margin: 0 0 5px;
  color: var(--text-faint);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
  text-transform: uppercase;
}

.article-workspace__title h1 {
  margin: 0;
  overflow: hidden;
  color: var(--text);
  font-size: 32px;
  font-weight: 750;
  letter-spacing: 0;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.article-workspace__title > span {
  display: block;
  margin-top: 6px;
  overflow: hidden;
  color: var(--text-faint);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.article-workspace__status {
  position: relative;
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 7px;
  padding-top: 2px;
}

.article-workspace__publish-shortcut {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 7px;
  margin-left: 4px;
  padding: 0 13px;
  border: 0;
  border-radius: 6px;
  background: var(--primary);
  color: white;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 22%, transparent);
}

.article-workspace__publish-shortcut:hover {
  filter: brightness(.92);
}

.article-workspace__tabs {
  position: sticky;
  z-index: 20;
  top: 84px;
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: stretch;
  gap: 4px;
  overflow-x: auto;
  overscroll-behavior-inline: contain;
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface) 94%, transparent);
  box-shadow: 0 10px 26px rgb(15 23 42 / 0.08);
  scrollbar-width: none;
  backdrop-filter: blur(12px);
}

.article-workspace__tabs::-webkit-scrollbar {
  display: none;
}

.article-workspace__tabs button {
  display: inline-flex;
  min-height: 42px;
  min-width: max-content;
  flex: 1 0 max-content;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 13px;
  border: 0 !important;
  border-radius: 6px !important;
  background: transparent;
  color: var(--text-soft);
  font-size: 14px;
  font-weight: 680;
  line-height: 1;
  white-space: nowrap;
  transition: background-color 140ms ease, color 140ms ease, box-shadow 140ms ease, transform 140ms ease;
}

.article-workspace__tabs button:hover {
  background: var(--surface-subtle);
  color: var(--text);
}

.article-workspace__tabs button.is-active {
  background: color-mix(in srgb, var(--primary-soft) 82%, var(--surface));
  color: var(--primary);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--primary) 24%, transparent),
    0 1px 0 color-mix(in srgb, var(--primary) 18%, transparent);
}

.article-workspace__tabs button:focus-visible {
  position: relative;
  z-index: 1;
}

.article-workspace__panel-stack {
  display: grid;
  gap: 18px;
}

.article-compose {
  border-radius: 8px !important;
  padding: 20px !important;
  overflow: hidden;
}

.article-compose__title input {
  min-height: 50px;
  padding-right: 14px;
  padding-left: 14px;
  font-size: 20px;
  font-weight: 650;
}

.article-compose__summaries {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.article-compose__summaries small {
  color: var(--text-faint);
  font-size: 11px;
  font-weight: 400;
}

.article-compose__summaries textarea {
  min-height: 104px;
}

.article-compose__advanced {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}

.article-compose__advanced > summary {
  display: flex;
  min-height: 54px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 13px;
  list-style: none;
  cursor: pointer;
}

.article-compose__advanced > summary::-webkit-details-marker { display: none; }
.article-compose__advanced > summary:hover { background: var(--surface-subtle); }
.article-compose__advanced > summary > span { display: grid; gap: 2px; }
.article-compose__advanced > summary strong { font-size: 13px; }
.article-compose__advanced > summary small { color: var(--text-faint); font-size: 11px; }
.article-compose__advanced[open] > summary { border-bottom: 1px solid var(--border); }
.article-compose__advanced[open] > summary svg { transform: rotate(180deg); }

.article-publish-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.article-publish__primary,
.article-publish__advanced { grid-column: 1 / -1; }

.article-publish__lead {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.article-publish__icon {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  place-items: center;
  border-radius: 10px;
  background: color-mix(in srgb, var(--primary-soft) 84%, var(--surface));
  color: var(--primary);
}

.article-publish__details {
  margin-top: 20px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
}

.article-publish__details > summary {
  display: flex;
  min-height: 52px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  list-style: none;
  cursor: pointer;
}

.article-publish__details > summary::-webkit-details-marker { display: none; }
.article-publish__details > summary:hover { background: var(--surface-subtle); }
.article-publish__details > summary > span { display: grid; gap: 2px; }
.article-publish__details > summary strong { font-size: 13px; }
.article-publish__details > summary small { color: var(--text-faint); font-size: 11px; font-weight: 400; }
.article-publish__details[open] > summary { border-bottom: 1px solid var(--border); }
.article-publish__details[open] > summary svg { transform: rotate(180deg); }

.article-publish__advanced {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  box-shadow: var(--shadow-sm);
}

.article-publish__advanced > summary {
  display: flex;
  min-height: 62px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  list-style: none;
  cursor: pointer;
}

.article-publish__advanced > summary::-webkit-details-marker { display: none; }
.article-publish__advanced > summary:hover { background: var(--surface-subtle); }
.article-publish__advanced > summary > span { display: grid; gap: 3px; }
.article-publish__advanced > summary strong { font-size: 14px; }
.article-publish__advanced > summary small { color: var(--text-faint); font-size: 12px; font-weight: 400; }
.article-publish__advanced[open] > summary { border-bottom: 1px solid var(--border); }
.article-publish__advanced[open] > summary svg { transform: rotate(180deg); }

.article-publish__schedule-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  padding: 14px 16px;
  background: var(--surface-subtle);
}

.article-publish__advanced-content {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 14px;
  padding: 16px;
  background: var(--surface-subtle);
}

.article-publish__option {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}

.article-publish__option > button,
.article-publish__controls > div:last-child { margin-top: auto; }

/* Shared treatment across every workspace tab. */
.article-workspace__panel-stack > article,
.article-workspace__panel-stack > section,
.article-workspace__panel-stack > form {
  overflow: hidden;
  border-radius: 10px !important;
}

.article-overview__summary > dl > div {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-subtle);
}

.article-overview__summary > dl svg {
  width: 18px;
  height: 18px;
}

.article-detail button.text-white {
  border-color: var(--primary);
  background: var(--primary);
  color: white;
  box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 15%, transparent);
}

.article-detail button.text-white:hover:not(:disabled) {
  border-color: var(--primary-hover);
  background: var(--primary-hover);
}

.article-compose > button:last-child {
  min-height: 44px;
  font-weight: 700;
}

.article-detail > header {
  display: none;
}

.article-detail > .mx-auto {
  display: block;
  max-width: none;
  padding: 0;
}

.article-detail > .mx-auto > :first-child {
  display: none;
}

.article-detail > .mx-auto > :last-child {
  width: 100%;
}

.article-detail .grid { display: grid; }
.article-detail .flex { display: flex; }
.article-detail .inline-flex { display: inline-flex; }
.article-detail .block { display: block; }
.article-detail .hidden { display: none; }
.article-detail .min-w-0 { min-width: 0; }
.article-detail .flex-1 { flex: 1 1 0%; }
.article-detail .flex-wrap { flex-wrap: wrap; }
.article-detail .items-center { align-items: center; }
.article-detail .items-start { align-items: flex-start; }
.article-detail .justify-between { justify-content: space-between; }
.article-detail .justify-end { justify-content: flex-end; }
.article-detail .justify-center { justify-content: center; }
.article-detail .self-end { align-self: flex-end; }
.article-detail .order-1 { order: 1; }
.article-detail .order-2 { order: 2; }

.article-detail .gap-2 { gap: 8px; }
.article-detail .gap-3 { gap: 12px; }
.article-detail .gap-4 { gap: 16px; }
.article-detail .gap-5 { gap: 20px; }
.article-detail .gap-6 { gap: 24px; }
.article-detail .space-y-1 > * + * { margin-top: 4px; }
.article-detail .space-y-2 > * + * { margin-top: 8px; }
.article-detail .space-y-3 > * + * { margin-top: 12px; }
.article-detail .space-y-4 > * + * { margin-top: 16px; }
.article-detail .space-y-5 > * + * { margin-top: 20px; }

.article-detail .mt-1 { margin-top: 4px; }
.article-detail .mt-2 { margin-top: 8px; }
.article-detail .mt-3 { margin-top: 12px; }
.article-detail .mt-4 { margin-top: 16px; }
.article-detail .mt-5 { margin-top: 20px; }
.article-detail .mb-1 { margin-bottom: 4px; }
.article-detail .p-3 { padding: 12px; }
.article-detail .p-4 { padding: 16px; }
.article-detail .p-5 { padding: 20px; }
.article-detail .p-6 { padding: 24px; }
.article-detail .px-2 { padding-right: 8px; padding-left: 8px; }
.article-detail .px-2\.5 { padding-right: 10px; padding-left: 10px; }
.article-detail .px-3 { padding-right: 12px; padding-left: 12px; }
.article-detail .px-4 { padding-right: 16px; padding-left: 16px; }
.article-detail .py-1 { padding-top: 4px; padding-bottom: 4px; }
.article-detail .py-1\.5 { padding-top: 6px; padding-bottom: 6px; }
.article-detail .py-2 { padding-top: 8px; padding-bottom: 8px; }
.article-detail .py-3 { padding-top: 12px; padding-bottom: 12px; }
.article-detail .pt-4 { padding-top: 16px; }

.article-detail .rounded-md { border-radius: 6px; }
.article-detail .rounded-lg { border-radius: 8px; }
.article-detail .rounded-full { border-radius: 999px; }
.article-detail .border { border: 1px solid var(--border); }
.article-detail .border-t { border-top: 1px solid var(--border); }
.article-detail .border-b { border-bottom: 1px solid var(--border); }
.article-detail .border-dashed { border-style: dashed; }
.article-detail .bg-white { background: var(--surface); }
.article-detail .shadow-sm { box-shadow: var(--shadow-sm); }

.article-detail article.rounded-lg,
.article-detail section.rounded-lg,
.article-detail form.rounded-lg,
.article-detail .empty-state,
.article-detail .rounded-lg.border {
  border-color: var(--border);
  background: var(--surface);
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.05), 0 16px 40px rgb(15 23 42 / 0.04);
}

.article-detail .article-compose > .flex:first-child,
.article-detail .article-publish__primary > .flex:first-child,
.article-detail form.rounded-lg > .flex:first-child,
.article-detail section.rounded-lg > .flex:first-child {
  margin: -20px -20px 0;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-subtle);
}

.article-detail h2,
.article-detail h3,
.article-detail p,
.article-detail dl,
.article-detail ol,
.article-detail ul {
  margin-bottom: 0;
}

.article-detail h2 {
  color: var(--text);
}

.article-detail .text-xs { font-size: 12px; line-height: 1.4; }
.article-detail .text-sm { font-size: 14px; line-height: 1.45; }
.article-detail .text-lg { font-size: 18px; line-height: 1.3; }
.article-detail .text-xl { font-size: 22px; line-height: 1.25; }
.article-detail .text-2xl { font-size: 26px; line-height: 1.2; }
.article-detail .font-medium { font-weight: 600; }
.article-detail .font-semibold { font-weight: 700; }
.article-detail .font-mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.article-detail .uppercase { text-transform: uppercase; }
.article-detail .tracking-normal { letter-spacing: 0; }
.article-detail .text-center { text-align: center; }

.article-detail .truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.article-detail .line-clamp-2 {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.article-detail .rounded-full {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  border: 1px solid color-mix(in srgb, currentColor 16%, transparent);
  font-weight: 700;
}

.article-detail dl {
  display: grid;
  gap: 12px;
}

.article-detail dt {
  color: var(--text-faint);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.article-detail dd {
  margin: 3px 0 0;
  color: var(--text);
}

.article-detail svg {
  flex: 0 0 auto;
}

.article-detail button,
.article-detail input,
.article-detail textarea,
.article-detail select {
  border: 1px solid var(--border-strong);
  border-radius: 6px;
}

.article-detail button {
  min-height: 36px;
  cursor: pointer;
  transition: background-color 140ms ease, border-color 140ms ease, color 140ms ease, transform 140ms ease;
}

.article-detail button:not(:disabled):active {
  transform: translateY(1px);
}

.article-detail button:disabled {
  cursor: not-allowed;
  opacity: .6;
}

.article-detail input,
.article-detail textarea,
.article-detail select {
  width: 100%;
  background: var(--surface);
  color: var(--text);
  box-shadow: inset 0 1px 1px rgb(15 23 42 / 0.035);
}

.article-detail input:focus,
.article-detail textarea:focus,
.article-detail select:focus {
  border-color: var(--primary);
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent);
}

.article-detail textarea {
  resize: vertical;
}

.article-detail pre {
  margin: 0;
  overflow: auto;
  border: 1px solid var(--border);
}

.article-detail .sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
}

@media (min-width: 640px) {
  .article-detail .sm\:grid-cols-2 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .article-detail .sm\:col-span-2 { grid-column: span 2 / span 2; }
}

@media (min-width: 768px) {
  .article-detail .md\:grid-cols-2 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .article-detail .md\:grid-cols-3 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}

@media (max-width: 820px) {
  .article-workspace__hero {
    display: grid;
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 20px;
  }

  .article-workspace__status {
    justify-content: flex-start;
    padding-top: 0;
  }

  .article-compose__summaries,
  .article-publish-grid {
    grid-template-columns: 1fr;
  }

  .article-publish__primary,
  .article-publish__advanced {
    grid-column: auto;
  }
}

@media (max-width: 620px) {
  .article-workspace__tabs {
    top: 76px;
  }

  .article-publish__lead {
    flex-wrap: wrap;
  }

}

@media (max-width: 560px) {
  .article-workspace__title h1 {
    font-size: 25px;
    white-space: normal;
  }

  .article-workspace__tabs {
    margin-right: -2px;
    margin-left: -2px;
  }

  .article-workspace__tabs button {
    flex: 0 0 auto;
    padding-right: 10px;
    padding-left: 10px;
  }

  .article-compose {
    padding: 16px !important;
  }

  .article-publish__schedule-form {
    grid-template-columns: 1fr;
  }

  .article-publish__primary > .mt-5 button {
    width: 100%;
  }
}
</style>
