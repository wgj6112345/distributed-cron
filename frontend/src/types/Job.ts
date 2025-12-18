// frontend/src/types/Job.ts
export interface Job {
    id: string;
    name: string;
    cron_expr: string;
    executor_type: 'http' | 'shell';
    executor: {
      url?: string;
      method?: string;
      command?: string;
    };
    concurrency_policy?: 'Allow' | 'Forbid';
    retry_policy?: {
      max_retries: number;
      backoff: string;
    };
    created_at: string;
    updated_at: string;
  }
  
  export interface ExecutionRecord {
    id: string;
    job_name: string;
    start_time: string;
    end_time: string;
    status: 'running' | 'success' | 'failed';
    output?: string;
    error?: string;
    retries_attempted: number;
    worker_id?: string;
  }
