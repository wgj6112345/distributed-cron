<!-- frontend/src/views/HomeView.vue -->
<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService } from '../services/apiService';
import { Job } from '../types/Job';
import JobForm from '../components/JobForm.vue';
import { Modal } from 'bootstrap'; // Import bootstrap's Modal type

// Reactive references
const jobs = ref<Job[]>([]);
const isLoading = ref(true);
const error = ref<string | null>(null);
const addJobModal = ref<any>(null); // To hold the modal instance

// Function to fetch jobs from the API
const fetchJobs = async () => {
  try {
    isLoading.value = true;
    jobs.value = await apiService.getJobs();
    error.value = null;
  } catch (err: any) {
    error.value = 'Failed to fetch jobs: ' + (err.message || 'Unknown error');
    console.error(err);
  } finally {
    isLoading.value = false;
  }
};

// Function to handle the delete button click
const handleDelete = async (jobName: string) => {
  if (!confirm(`Are you sure you want to delete job "${jobName}"?`)) {
    return;
  }
  try {
    await apiService.deleteJob(jobName);
    await fetchJobs(); // Refresh the job list after deletion
  } catch (err: any) {
    alert('Failed to delete job: ' + err.message);
    console.error(err);
  }
};

// Handle job created event from JobForm
const handleJobCreated = () => {
  // Programmatically hide the modal
  const modalInstance = Modal.getInstance(addJobModal.value);
  if (modalInstance) {
    modalInstance.hide();
  }
  fetchJobs(); // Refresh list
};

// Fetch jobs when the component is mounted
onMounted(() => {
  fetchJobs();
});
</script>

<template>
  <main class="container mt-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h1>Distributed Cron Jobs</h1>
      <button class="btn btn-success" data-bs-toggle="modal" data-bs-target="#addJobModal">
        &#43; Add New Job
      </button>
    </div>

    <!-- Job List Table -->
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
        <table class="table table-striped table-hover align-middle mb-0">
          <!-- ... (table head remains the same) ... -->
          <thead>
            <tr>
              <th>Name</th>
              <th>Cron Expression</th>
              <th>Executor Type</th>
              <th>Details</th>
              <th>Concurrency</th>
              <th>Retries</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="jobs.length === 0">
              <td colspan="7" class="text-center text-muted">No jobs found. Click "Add New Job" to create one.</td>
            </tr>
            <tr v-for="job in jobs" :key="job.id">
              <td><strong>{{ job.name }}</strong></td>
              <td><code>{{ job.cron_expr }}</code></td>
              <td>
                <span class="badge" :class="job.executor_type === 'http' ? 'bg-primary' : 'bg-secondary'">
                  {{ job.executor_type.toUpperCase() }}
                </span>
              </td>
              <td>
                <small v-if="job.executor_type === 'http'">
                  <code>{{ job.executor.method }}</code> {{ job.executor.url }}
                </small>
                <small v-else-if="job.executor_type === 'shell'">
                  <code>{{ job.executor.command }}</code>
                </small>
              </td>
              <td>
                <span class="badge" :class="job.concurrency_policy === 'Allow' ? 'bg-info text-dark' : 'bg-warning text-dark'">
                  {{ job.concurrency_policy }}
                </span>
              </td>
              <td>
                <small v-if="job.retry_policy && job.retry_policy.max_retries > 0">{{ job.retry_policy.max_retries }} ({{ job.retry_policy.backoff }})</small>
                <small v-else>None</small>
              </td>
                          <td>
                            <RouterLink :to="{ name: 'job-history', params: { jobName: job.name } }" class="btn btn-sm btn-outline-secondary me-2">
                              History
                            </RouterLink>
                            <button class="btn btn-sm btn-danger" @click="handleDelete(job.name)">
                              Delete
                            </button>
                          </td>            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Bootstrap Modal for the Job Form -->
    <div class="modal fade" id="addJobModal" tabindex="-1" aria-labelledby="addJobModalLabel" aria-hidden="true" ref="addJobModal">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="addJobModalLabel">Create a New Job</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
          </div>
          <div class="modal-body">
            <JobForm @jobCreated="handleJobCreated" @closeForm="() => Modal.getInstance(addJobModal)?.hide()" />
          </div>
        </div>
      </div>
    </div>
  </main>
</template>