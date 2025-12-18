// frontend/src/services/apiService.ts
import axios from 'axios';
import type { Job } from '../types/Job'; // Assuming Job type is now in @/types/Job

// Define the base URL of our Go backend API
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  headers: {
    'Content-Type': 'application/json',
  },
});

export type SaveJobPayload = Omit<Job, 'id' | 'created_at' | 'updated_at'>;

export const apiService = {
  // Fetch all jobs
  async getJobs(): Promise<Job[]> {
    const response = await apiClient.get('/jobs/');
    return response.data || [];
  },

  // Delete a job by its name
  async deleteJob(name: string): Promise<void> {
    await apiClient.delete(`/jobs/${name}`);
  },

  // Create/update a job
  async saveJob(jobData: SaveJobPayload): Promise<Job> {
    const response = await apiClient.post('/jobs/', jobData);
    return response.data;
  },

  // Fetch job history
  async getJobHistory(jobName: string, page: number = 1, pageSize: number = 20): Promise<any[]> {
    const response = await apiClient.get(`/jobs/${jobName}/history`, {
      params: { page, pageSize }
    });
    return response.data || [];
  }
};
