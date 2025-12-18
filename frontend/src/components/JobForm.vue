<!-- frontend/src/components/JobForm.vue -->
<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { apiService, type SaveJobPayload } from '../services/apiService';

const emit = defineEmits(['jobCreated', 'closeForm']);

// Form data reactive references
const jobName = ref('');
const cronExpr = ref('');
const executorType = ref<'http' | 'shell'>('http');
const httpUrl = ref('');
const httpMethod = ref('GET');
const shellCommand = ref('');
const concurrencyPolicy = ref<'Allow' | 'Forbid'>('Allow');
const maxRetries = ref(0);
const backoff = ref('0s'); // e.g., "1s", "30s"

const isLoading = ref(false);
const error = ref<string | null>(null);
const successMessage = ref<string | null>(null);

// Reset form fields
const resetForm = () => {
  jobName.value = '';
  cronExpr.value = '';
  executorType.value = 'http';
  httpUrl.value = '';
  httpMethod.value = 'GET';
  shellCommand.value = '';
  concurrencyPolicy.value = 'Allow';
  maxRetries.value = 0;
  backoff.value = '0s';
  error.value = null;
  successMessage.value = null;
};

// Computed property to determine if HTTP fields should be shown
const showHttpFields = computed(() => executorType.value === 'http');
// Computed property to determine if Shell fields should be shown
const showShellFields = computed(() => executorType.value === 'shell');

// Watch for changes in executorType to clear irrelevant fields
watch(executorType, (newValue) => {
  if (newValue === 'http') {
    shellCommand.value = '';
  } else {
    httpUrl.value = '';
    httpMethod.value = 'GET';
  }
});

const handleSubmit = async () => {
  error.value = null;
  successMessage.value = null;
  isLoading.value = true;

  // Basic validation (more comprehensive validation from backend)
  if (!jobName.value || !cronExpr.value || !executorType.value) {
    error.value = 'Please fill in all required fields.';
    isLoading.value = false;
    return;
  }

  const payload: SaveJobPayload = {
    name: jobName.value,
    cron_expr: cronExpr.value,
    executor_type: executorType.value,
    executor: {}, // Initialize executor object
    concurrency_policy: concurrencyPolicy.value,
  };

  if (executorType.value === 'http') {
    if (!httpUrl.value) {
      error.value = 'HTTP URL is required.';
      isLoading.value = false;
      return;
    }
    payload.executor.url = httpUrl.value;
    payload.executor.method = httpMethod.value;
  } else if (executorType.value === 'shell') {
    if (!shellCommand.value) {
      error.value = 'Shell command is required.';
      isLoading.value = false;
      return;
    }
    payload.executor.command = shellCommand.value;
  }

  // Only add retry_policy if maxRetries > 0 or backoff is not "0s"
  if (maxRetries.value > 0 || backoff.value !== '0s') {
    payload.retry_policy = {
      max_retries: maxRetries.value,
      backoff: backoff.value,
    };
  }

  try {
    const newJob = await apiService.saveJob(payload);
    successMessage.value = `Job "${newJob.name}" created successfully!`;
    resetForm();
    emit('jobCreated'); // Notify parent component that a new job was created
  } catch (err: any) {
    if (err.response && err.response.data && err.response.data.details) {
      error.value = `Validation failed: ${err.response.data.details.join(', ')}`;
    } else if (err.response && err.response.data && err.response.data.error) {
        error.value = `Error: ${err.response.data.error}`;
    }
    else {
      error.value = 'Failed to create job: ' + (err.message || 'Unknown error');
    }
    console.error(err);
  } finally {
    isLoading.value = false;
  }
};
</script>

<template>
  <div class="card mt-4">
    <div class="card-header">
      <h5 class="mb-0">Create New Cron Job</h5>
    </div>
    <div class="card-body">
      <form @submit.prevent="handleSubmit">
        <div class="mb-3">
          <label for="jobName" class="form-label">Job Name</label>
          <input type="text" class="form-control" id="jobName" v-model="jobName" required>
        </div>
        <div class="mb-3">
          <label for="cronExpr" class="form-label">Cron Expression</label>
          <input type="text" class="form-control" id="cronExpr" v-model="cronExpr" placeholder="e.g., */5 * * * * *" required>
          <div class="form-text">Supports seconds. e.g., */5 * * * * * (every 5 seconds)</div>
        </div>

        <div class="mb-3">
          <label for="executorType" class="form-label">Executor Type</label>
          <select class="form-select" id="executorType" v-model="executorType">
            <option value="http">HTTP</option>
            <option value="shell">Shell Command</option>
          </select>
        </div>

        <div v-if="showHttpFields" class="card card-body bg-light mb-3">
          <h6>HTTP Executor Details</h6>
          <div class="mb-3">
            <label for="httpUrl" class="form-label">URL</label>
            <input type="url" class="form-control" id="httpUrl" v-model="httpUrl" required>
          </div>
          <div class="mb-3">
            <label for="httpMethod" class="form-label">Method</label>
            <select class="form-select" id="httpMethod" v-model="httpMethod">
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
            </select>
          </div>
        </div>

        <div v-if="showShellFields" class="card card-body bg-light mb-3">
          <h6>Shell Executor Details</h6>
          <div class="mb-3">
            <label for="shellCommand" class="form-label">Command</label>
            <input type="text" class="form-control" id="shellCommand" v-model="shellCommand" placeholder="e.g., date" required>
          </div>
        </div>

        <div class="mb-3">
          <label for="concurrencyPolicy" class="form-label">Concurrency Policy</label>
          <select class="form-select" id="concurrencyPolicy" v-model="concurrencyPolicy">
            <option value="Allow">Allow Concurrent Runs</option>
            <option value="Forbid">Forbid Concurrent Runs (Skip if running)</option>
          </select>
        </div>

        <div class="card card-body bg-light mb-3">
          <h6>Retry Policy (Optional)</h6>
          <div class="mb-3">
            <label for="maxRetries" class="form-label">Max Retries</label>
            <input type="number" class="form-control" id="maxRetries" v-model.number="maxRetries" min="0" max="10">
            <div class="form-text">Number of times to retry on failure (0 for no retries). Max 10.</div>
          </div>
          <div class="mb-3" v-if="maxRetries > 0">
            <label for="backoff" class="form-label">Retry Backoff Duration</label>
            <input type="text" class="form-control" id="backoff" v-model="backoff" placeholder="e.g., 1s, 30s, 1m">
            <div class="form-text">Time to wait before retrying (e.g., "5s", "1m").</div>
          </div>
        </div>

        <div v-if="error" class="alert alert-danger">{{ error }}</div>
        <div v-if="successMessage" class="alert alert-success">{{ successMessage }}</div>

        <button type="submit" class="btn btn-primary" :disabled="isLoading">
          <span v-if="isLoading" class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
          {{ isLoading ? 'Creating...' : 'Create Job' }}
        </button>
        <button type="button" class="btn btn-secondary ms-2" @click="emit('closeForm')">Cancel</button>
      </form>
    </div>
  </div>
</template>
