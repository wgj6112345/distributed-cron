<!-- frontend/src/views/HistoryView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService } from '../services/apiService';
import type { ExecutionRecord } from '../types/Job';
import { RouterLink } from 'vue-router';

const props = defineProps<{
  jobName: string;
}>();

const records = ref<ExecutionRecord[]>([]);
const isLoading = ref(true);
const error = ref<string | null>(null);

const fetchHistory = async () => {
  try {
    isLoading.value = true;
    records.value = await apiService.getJobHistory(props.jobName);
    error.value = null;
  } catch (err: any) {
    error.value = `Failed to fetch history for job "${props.jobName}": ${err.message}`;
    console.error(err);
  } finally {
    isLoading.value = false;
  }
};

const formatTime = (timeStr: string) => {
  if (!timeStr || timeStr.startsWith('0001-01-01')) return 'N/A';
  return new Date(timeStr).toLocaleString();
};

onMounted(() => {
  fetchHistory();
});
</script>

<template>
  <main class="container mt-4">
    <nav aria-label="breadcrumb">
      <ol class="breadcrumb">
        <li class="breadcrumb-item"><RouterLink to="/">All Jobs</RouterLink></li>
        <li class="breadcrumb-item active" aria-current="page">Execution History</li>
      </ol>
    </nav>

    <h1 class="mb-4">Execution History for <code class="text-primary">{{ jobName }}</code></h1>

    <div v-if="isLoading" class="text-center mt-5">
      <div class="spinner-border" role="status">
        <span class="visually-hidden">Loading...</span>
      </div>
    </div>

    <div v-else-if="error" class="alert alert-danger">
      {{ error }}
    </div>

    <div v-else class="card">
      <div class="card-body">
        <table class="table table-sm table-hover align-middle">
          <thead>
            <tr>
              <th>Status</th>
              <th>Start Time</th>
              <th>End Time</th>
              <th>Error</th>
              <th>Output</th>
              <th>Worker ID</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="records.length === 0">
              <td colspan="6" class="text-center text-muted">No execution history found for this job.</td>
            </tr>
            <tr v-for="record in records" :key="record.id">
              <td>
                <span class="badge" :class="{
                  'bg-success': record.status === 'success',
                  'bg-danger': record.status === 'failed',
                  'bg-info': record.status === 'running'
                }">{{ record.status }}</span>
              </td>
              <td>{{ formatTime(record.start_time) }}</td>
              <td>{{ formatTime(record.end_time) }}</td>
              <td><small class="text-danger"><code>{{ record.error }}</code></small></td>
              <td>
                <pre v-if="record.output" class="bg-light p-2 rounded" style="max-height: 100px; overflow-y: auto;"><code>{{ record.output }}</code></pre>
              </td>
              <td><small class="text-muted">{{ record.worker_id }}</small></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </main>
</template>
