import type { IssueRecord } from '../data';
import { issuesApi } from './issuesApi';

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

class CenterTaskExceptionsApi implements TaskExceptionsApi {
  async listByJobIds(jobIds: string[]): Promise<TaskExceptionRecord[]> {
    const issues = await issuesApi.listByJobIds(jobIds);
    return issues.map(mapIssueToTaskException);
  }
}

export const taskExceptionsApi: TaskExceptionsApi = new CenterTaskExceptionsApi();

function mapIssueToTaskException(issue: IssueRecord): TaskExceptionRecord {
  return {
    id: issue.id,
    jobId: issue.taskId ?? issue.source.taskId ?? '',
    jobItemId: issue.taskItemId ?? issue.source.taskItemId,
    title: issue.title,
    summary: issue.summary,
    severity: issue.severity,
    createdAt: issue.createdAt,
  };
}
