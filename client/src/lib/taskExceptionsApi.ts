export type TaskExceptionRecord = {
  id: string;
  jobId: string;
  jobItemId?: string;
  title: string;
  summary: string;
  severity: 'info' | 'warning' | 'critical' | 'success';
  createdAt: string;
};

export interface TaskExceptionsApi {
  listByJobIds(jobIds: string[]): Promise<TaskExceptionRecord[]>;
}

class NoopTaskExceptionsApi implements TaskExceptionsApi {
  async listByJobIds(_jobIds: string[]): Promise<TaskExceptionRecord[]> {
    return [];
  }
}

export const taskExceptionsApi: TaskExceptionsApi = new NoopTaskExceptionsApi();
