<script setup lang="ts">
import { computed } from 'vue'
import { formatDateTime } from '../lib/format'
import { audit, auditLoading, openFinding, refreshAudit, switchView } from '../stores/session'
import type { AuditSeverity } from '../types'
import Icon from './Icon.vue'

const report = computed(() => audit.value)
const score = computed(() => report.value?.score ?? 0)

const verdict = computed(() => {
  const value = score.value
  if (value >= 90) return { label: 'In good shape', tone: 'good' as const }
  if (value >= 70) return { label: 'A few things to fix', tone: 'ok' as const }
  if (value >= 40) return { label: 'Needs attention', tone: 'warn' as const }
  return { label: 'At risk', tone: 'bad' as const }
})

const checks = computed(() => {
  const current = report.value
  if (!current) return []
  return [
    { key: 'weak', label: 'Weak passwords', count: current.weak, icon: 'alert' as const, hint: 'Rated weak or fair' },
    { key: 'reused', label: 'Reused passwords', count: current.reused, icon: 'copy' as const, hint: 'Shared between items' },
    { key: 'stale', label: 'Old passwords', count: current.stale, icon: 'clock' as const, hint: 'Not changed in a year' },
    { key: 'noTotp', label: 'No second factor', count: current.noTotp, icon: 'shield' as const, hint: 'Logins with a URL' },
    { key: 'cards', label: 'Cards', count: current.cards, icon: 'card' as const, hint: 'Expired or expiring' },
  ]
})

function severityLabel(severity: AuditSeverity): string {
  return severity === 'high' ? 'High' : severity === 'medium' ? 'Medium' : 'Advisory'
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <div class="grow">
        <div class="eyebrow">Security check</div>
        <h2 class="title">Password health</h2>
        <p class="faint lede">
          Every verdict below is computed on this device from the unlocked vault. There is no breach-corpus
          lookup, no account and no network request — weak, reused and stale passwords are found by comparing
          your own items against each other.
        </p>
      </div>
      <div class="head-actions">
        <button class="btn quiet" type="button" :disabled="auditLoading" @click="refreshAudit">
          <Icon name="refresh" :size="14" />
          Re-run
        </button>
        <button class="btn quiet" type="button" @click="switchView('items')">
          <Icon name="chevronLeft" :size="14" />
          Items
        </button>
      </div>
    </header>

    <div v-if="!report" class="skeleton card-lite">Running the health check…</div>

    <template v-else>
      <div class="score-card" :class="verdict.tone">
        <div class="score">
          <span class="value">{{ report.score }}</span>
          <span class="out-of">/ 100</span>
        </div>
        <div class="grow">
          <div class="verdict">{{ verdict.label }}</div>
          <div class="faint score-note">
            {{ report.items }} {{ report.items === 1 ? 'item' : 'items' }} checked
            <template v-if="report.affected > 0"> · {{ report.affected }} with something to fix</template>
            <template v-if="report.trashed > 0"> · {{ report.trashed }} in the trash</template>
          </div>
          <div class="meter"><span :style="{ width: `${report.score}%` }" /></div>
        </div>
        <div class="faint stamp">{{ formatDateTime(report.checkedAt) }}</div>
      </div>

      <div class="checks">
        <div v-for="check in checks" :key="check.key" class="check" :class="{ clear: check.count === 0 }">
          <Icon :name="check.icon" :size="16" />
          <div class="grow">
            <div class="check-count">{{ check.count }}</div>
            <div class="check-label">{{ check.label }}</div>
            <div class="faint check-hint">{{ check.hint }}</div>
          </div>
        </div>
      </div>

      <div v-if="report.findings.length === 0" class="empty">
        <Icon name="shieldCheck" :size="34" />
        <p>Nothing to fix. Every stored password is unique, and none of them look weak.</p>
      </div>

      <div v-else class="findings">
        <div class="eyebrow">Findings</div>
        <button
          v-for="(finding, index) in report.findings"
          :key="`${finding.itemId}-${finding.kind}-${index}`"
          type="button"
          class="finding"
          @click="openFinding(finding.itemId)"
        >
          <span class="dot" :class="finding.severity" />
          <span class="grow">
            <span class="finding-title">{{ finding.title }}</span>
            <span class="faint finding-detail">{{ finding.detail }}</span>
          </span>
          <span class="sev">{{ severityLabel(finding.severity) }}</span>
          <Icon name="chevronRight" :size="14" />
        </button>
      </div>

      <p class="faint footnote">
        A strong master password is still the most important factor: the audit only sees what is already inside
        the vault. Age thresholds use the recorded password changes, not the last time an item was edited.
      </p>
    </template>
  </section>
</template>

<style scoped>
.panel {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 20px 26px 32px;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}

.head-actions {
  display: flex;
  gap: 6px;
}

.title {
  font-size: 19px;
  font-weight: 700;
  margin: 2px 0 4px;
}

.lede {
  font-size: 12.5px;
  line-height: 1.6;
  max-width: 62ch;
}

.card-lite {
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  font-size: 13px;
  color: var(--text-mute);
}

.score-card {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 16px 18px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--surface);
  box-shadow: var(--shadow-1);
}

.score-card.good {
  border-color: color-mix(in srgb, var(--good) 38%, transparent);
  background: linear-gradient(180deg, var(--good-soft), transparent 70%), var(--surface);
}

.score-card.ok {
  border-color: color-mix(in srgb, var(--accent) 38%, transparent);
  background: linear-gradient(180deg, var(--accent-soft), transparent 70%), var(--surface);
}

.score-card.warn {
  border-color: color-mix(in srgb, var(--warn) 38%, transparent);
  background: linear-gradient(180deg, var(--warn-soft), transparent 70%), var(--surface);
}

.score-card.bad {
  border-color: color-mix(in srgb, var(--bad) 38%, transparent);
  background: linear-gradient(180deg, var(--bad-soft), transparent 70%), var(--surface);
}

.score {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.score .value {
  font-size: 38px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1;
}

.score .out-of {
  font-size: 12px;
  color: var(--text-mute);
}

.verdict {
  font-size: 14px;
  font-weight: 700;
}

.score-note {
  font-size: 12px;
  margin-top: 2px;
}

.meter {
  margin-top: 9px;
  height: 5px;
  border-radius: 99px;
  background: var(--surface-3);
  overflow: hidden;
}

.meter span {
  display: block;
  height: 100%;
  border-radius: 99px;
  background: linear-gradient(90deg, var(--accent), var(--accent-strong));
  transition: width 0.3s ease;
}

.stamp {
  font-size: 11.5px;
  align-self: flex-start;
}

.checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(158px, 1fr));
  gap: 8px;
  margin: 14px 0 20px;
}

.check {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--warn);
}

.check.clear {
  color: var(--good);
}

.check-count {
  font-size: 18px;
  font-weight: 700;
  line-height: 1.1;
  color: var(--text);
}

.check-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-dim);
}

.check-hint {
  font-size: 11px;
}

.findings {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.finding {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 11px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: background 0.14s ease, border-color 0.14s ease;
}

.finding:hover {
  background: var(--surface-2);
  border-color: var(--border-strong);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
  background: var(--text-mute);
}

.dot.high {
  background: var(--bad);
  box-shadow: 0 0 0 3px var(--bad-soft);
}

.dot.medium {
  background: var(--warn);
  box-shadow: 0 0 0 3px var(--warn-soft);
}

.dot.low {
  background: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.finding-title {
  display: block;
  font-size: 13px;
  font-weight: 600;
}

.finding-detail {
  display: block;
  font-size: 11.5px;
  line-height: 1.45;
}

.sev {
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-mute);
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 34px 20px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius);
  color: var(--good);
  text-align: center;
  font-size: 13px;
}

.footnote {
  margin-top: 20px;
  font-size: 11.5px;
  line-height: 1.6;
  max-width: 74ch;
}
</style>
