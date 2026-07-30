<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <h2>Editorial calendar</h2>
        <p>Scheduled releases, recent publications, and drafts that still need a date.</p>
      </div>
      <div class="calendar-toolbar">
        <button class="icon-button surface" type="button" title="Previous month" aria-label="Previous month" @click="moveMonth(-1)">
          <ChevronLeft :size="17" />
        </button>
        <button class="button button--compact" type="button" @click="goToToday">Today</button>
        <button class="icon-button surface" type="button" title="Next month" aria-label="Next month" @click="moveMonth(1)">
          <ChevronRight :size="17" />
        </button>
      </div>
    </div>

    <div class="metric-grid">
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Scheduled</span><CalendarClock :size="17" /></div>
        <p class="metric-card__value">{{ scheduled.length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Published this month</span><CircleCheck :size="17" /></div>
        <p class="metric-card__value">{{ publishedThisMonth }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>In review</span><ListChecks :size="17" /></div>
        <p class="metric-card__value">{{ inReview.length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Unscheduled drafts</span><Inbox :size="17" /></div>
        <p class="metric-card__value">{{ unscheduled.length }}</p>
      </article>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>

    <div v-if="pending" class="loading-surface surface">
      <LoaderCircle class="spin" :size="18" />
      Loading calendar
    </div>

    <div v-else class="calendar-layout">
      <section class="calendar surface" aria-label="Monthly editorial calendar">
        <header class="calendar__header">
          <div>
            <p class="calendar__eyebrow">{{ currentYear }}</p>
            <h3>{{ currentMonthLabel }}</h3>
          </div>
          <div class="calendar__legend" aria-label="Calendar legend">
            <span><i class="legend-dot legend-dot--scheduled" />Scheduled</span>
            <span><i class="legend-dot legend-dot--published" />Published</span>
          </div>
        </header>

        <div class="calendar__weekdays" aria-hidden="true">
          <span v-for="day in weekdays" :key="day">{{ day }}</span>
        </div>
        <div class="calendar__grid">
          <div
            v-for="day in calendarDays"
            :key="day.key"
            class="calendar-day"
            :class="{ 'calendar-day--muted': !day.inMonth, 'calendar-day--today': day.isToday }"
          >
            <time :datetime="day.key">{{ day.date.getDate() }}</time>
            <div class="calendar-day__events">
              <NuxtLink
                v-for="event in day.events.slice(0, 3)"
                :key="`${event.id}-${event.kind}`"
                class="calendar-event"
                :class="`calendar-event--${event.kind}`"
                :to="`/projects/${projectID}/articles/${event.id}`"
                :title="event.title"
              >
                <span>{{ event.title }}</span>
              </NuxtLink>
              <span v-if="day.events.length > 3" class="calendar-day__more">+{{ day.events.length - 3 }} more</span>
            </div>
          </div>
        </div>
      </section>

      <aside class="calendar-rail">
        <section class="surface rail-panel">
          <div class="rail-panel__heading">
            <div>
              <span>Next up</span>
              <h3>Scheduled</h3>
            </div>
            <span class="status-pill status-pill--success">{{ scheduled.length }}</span>
          </div>
          <div v-if="scheduled.length" class="rail-list">
            <NuxtLink
              v-for="article in scheduled.slice(0, 6)"
              :key="article.id"
              class="rail-list__item"
              :to="`/projects/${projectID}/articles/${article.id}`"
            >
              <span class="rail-list__date">
                <strong>{{ dayNumber(article.scheduledForUtc) }}</strong>
                <small>{{ monthShort(article.scheduledForUtc) }}</small>
              </span>
              <span class="rail-list__copy">
                <strong>{{ article.title }}</strong>
                <small>{{ formatTime(article.scheduledForUtc) }} · {{ article.locale.toUpperCase() }}</small>
              </span>
            </NuxtLink>
          </div>
          <div v-else class="rail-empty">No scheduled articles</div>
        </section>

        <section class="surface rail-panel">
          <div class="rail-panel__heading">
            <div>
              <span>Planning</span>
              <h3>Needs a date</h3>
            </div>
          </div>
          <div v-if="unscheduled.length" class="rail-list rail-list--plain">
            <NuxtLink
              v-for="article in unscheduled.slice(0, 6)"
              :key="article.id"
              class="rail-list__item"
              :to="`/projects/${projectID}/articles/${article.id}`"
            >
              <span class="article-type-icon"><FileText :size="16" /></span>
              <span class="rail-list__copy">
                <strong>{{ article.title }}</strong>
                <small>{{ labelize(article.editorialState) }}</small>
              </span>
            </NuxtLink>
          </div>
          <div v-else class="rail-empty">All active drafts are planned</div>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  FileText,
  Inbox,
  ListChecks,
  LoaderCircle
} from 'lucide-vue-next'
import type { AdminArticle } from '~/composables/useAdminApi'

type CalendarEvent = AdminArticle & { kind: 'scheduled' | 'published' }

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const articles = ref<AdminArticle[]>([])
const pending = ref(true)
const errorMessage = ref('')
const visibleMonth = ref(new Date(new Date().getFullYear(), new Date().getMonth(), 1))
const weekdays = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const scheduled = computed(() => articles.value
  .filter(article => article.publicationState === 'scheduled' && article.scheduledForUtc)
  .sort((a, b) => dateValue(a.scheduledForUtc) - dateValue(b.scheduledForUtc)))
const inReview = computed(() => articles.value.filter(article => article.editorialState === 'in_review'))
const unscheduled = computed(() => articles.value.filter(article =>
  ['draft', 'changes_requested', 'approved'].includes(article.editorialState)
  && !article.scheduledForUtc
  && article.publicationState !== 'published'
))
const currentMonthLabel = computed(() => visibleMonth.value.toLocaleDateString(undefined, { month: 'long' }))
const currentYear = computed(() => visibleMonth.value.getFullYear())
const publishedThisMonth = computed(() => articles.value.filter(article => {
  const date = parseDate(article.publishedAt)
  return date && date.getFullYear() === currentYear.value && date.getMonth() === visibleMonth.value.getMonth()
}).length)

const calendarEvents = computed<CalendarEvent[]>(() => articles.value.flatMap(article => {
  const events: CalendarEvent[] = []
  if (article.scheduledForUtc) events.push({ ...article, kind: 'scheduled' })
  if (article.publishedAt) events.push({ ...article, kind: 'published' })
  return events
}))

const calendarDays = computed(() => {
  const first = new Date(currentYear.value, visibleMonth.value.getMonth(), 1)
  const mondayOffset = (first.getDay() + 6) % 7
  const start = new Date(first)
  start.setDate(first.getDate() - mondayOffset)
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start)
    date.setDate(start.getDate() + index)
    const key = localDateKey(date)
    return {
      key,
      date,
      inMonth: date.getMonth() === visibleMonth.value.getMonth(),
      isToday: key === localDateKey(new Date()),
      events: calendarEvents.value.filter(event => localDateKey(parseDate(event.kind === 'scheduled' ? event.scheduledForUtc : event.publishedAt)) === key)
    }
  })
})

onMounted(loadArticles)

async function loadArticles() {
  pending.value = true
  errorMessage.value = ''
  try {
    articles.value = (await api.listArticles(projectID.value)).data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load the editorial calendar.')
  } finally {
    pending.value = false
  }
}

function moveMonth(offset: number) {
  visibleMonth.value = new Date(currentYear.value, visibleMonth.value.getMonth() + offset, 1)
}

function goToToday() {
  visibleMonth.value = new Date(new Date().getFullYear(), new Date().getMonth(), 1)
}

function parseDate(value?: string | Date | null) {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function localDateKey(value?: string | Date | null) {
  const date = parseDate(value)
  if (!date) return ''
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function dateValue(value?: string) {
  return parseDate(value)?.getTime() || Number.MAX_SAFE_INTEGER
}

function dayNumber(value?: string) {
  return parseDate(value)?.getDate() || '–'
}

function monthShort(value?: string) {
  return parseDate(value)?.toLocaleDateString(undefined, { month: 'short' }) || ''
}

function formatTime(value?: string) {
  return parseDate(value)?.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }) || ''
}
</script>

<style scoped>
.calendar-toolbar { display: flex; align-items: center; gap: 7px; }
.calendar-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; align-items: start; }
.calendar { overflow: hidden; }
.calendar__header { display: flex; min-height: 72px; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 18px; border-bottom: 1px solid var(--border); }
.calendar__header h3 { margin: 1px 0 0; font-size: 18px; }
.calendar__eyebrow { margin: 0; color: var(--text-soft); font-size: 11px; }
.calendar__legend { display: flex; flex-wrap: wrap; gap: 12px; color: var(--text-soft); font-size: 11px; }
.calendar__legend span { display: inline-flex; align-items: center; gap: 6px; }
.legend-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--blue); }
.legend-dot--scheduled { background: var(--amber); }
.calendar__weekdays { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); border-bottom: 1px solid var(--border); background: var(--surface-subtle); }
.calendar__weekdays span { padding: 8px 10px; color: var(--text-soft); font-size: 10px; font-weight: 650; text-transform: uppercase; }
.calendar__grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); }
.calendar-day { min-width: 0; min-height: 116px; padding: 8px; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); background: var(--surface); }
.calendar-day:nth-child(7n) { border-right: 0; }
.calendar-day:nth-last-child(-n + 7) { border-bottom: 0; }
.calendar-day > time { display: grid; width: 24px; height: 24px; place-items: center; color: var(--text-soft); font-size: 11px; font-weight: 600; }
.calendar-day--muted { background: var(--surface-subtle); opacity: .64; }
.calendar-day--today > time { border-radius: 50%; background: var(--primary); color: white; }
.calendar-day__events { display: grid; gap: 4px; margin-top: 5px; }
.calendar-event { display: block; overflow: hidden; padding: 4px 6px; border-left: 2px solid var(--blue); border-radius: 3px; background: var(--blue-soft); color: var(--text); font-size: 10px; font-weight: 600; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
.calendar-event--scheduled { border-left-color: var(--amber); background: var(--amber-soft); }
.calendar-day__more { color: var(--text-soft); font-size: 9px; }
.calendar-rail { display: grid; gap: 14px; }
.rail-panel { overflow: hidden; }
.rail-panel__heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 14px; border-bottom: 1px solid var(--border); }
.rail-panel__heading span { color: var(--text-soft); font-size: 10px; }
.rail-panel__heading h3 { margin: 1px 0 0; font-size: 15px; }
.rail-list__item { display: grid; grid-template-columns: 38px minmax(0, 1fr); align-items: center; gap: 10px; padding: 10px 14px; border-bottom: 1px solid var(--border); color: inherit; text-decoration: none; }
.rail-list__item:last-child { border-bottom: 0; }
.rail-list__item:hover { background: var(--surface-subtle); }
.rail-list__date { display: grid; height: 38px; place-items: center; align-content: center; border-radius: 6px; background: var(--primary-soft); color: var(--primary); line-height: 1; }
.rail-list__date strong { font-size: 14px; }
.rail-list__date small { margin-top: 3px; font-size: 8px; text-transform: uppercase; }
.rail-list__copy { display: flex; min-width: 0; flex-direction: column; }
.rail-list__copy strong, .rail-list__copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rail-list__copy strong { font-size: 11px; }
.rail-list__copy small { margin-top: 3px; color: var(--text-soft); font-size: 9px; text-transform: capitalize; }
.article-type-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 6px; background: var(--surface-subtle); color: var(--text-soft); }
.rail-empty { padding: 18px 14px; color: var(--text-soft); font-size: 11px; }
.loading-surface { display: flex; min-height: 120px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) { .calendar-layout { grid-template-columns: 1fr; } .calendar-rail { grid-template-columns: 1fr 1fr; } }
@media (max-width: 760px) { .calendar { overflow-x: auto; } .calendar__header, .calendar__weekdays, .calendar__grid { min-width: 720px; } .calendar-rail { grid-template-columns: 1fr; } }
</style>
