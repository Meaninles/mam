export type SystemRuntimeLightTone = 'success' | 'warning' | 'critical';
export type CenterRuntimeStatus = 'ready' | 'not_ready' | 'unreachable';
export type AgentRuntimeStatus = 'online' | 'heartbeat_timeout' | 'unknown';

export type SystemRuntimeSummary = {
  centerStatus: CenterRuntimeStatus;
  agentStatus: AgentRuntimeStatus;
  lightTone: SystemRuntimeLightTone;
};

type RuntimeStatusResponse = {
  data?: {
    status?: string;
    agents?: Array<{
      status?: string;
    }>;
  };
};

export async function fetchSystemRuntimeStatus(centerBaseUrl: string): Promise<SystemRuntimeSummary> {
  try {
    const response = await fetch(`${centerBaseUrl}/api/runtime/status`);
    if (!response.ok) {
      return {
        centerStatus: 'unreachable',
        agentStatus: 'unknown',
        lightTone: 'critical',
      };
    }

    const payload = (await response.json()) as RuntimeStatusResponse;
    const centerStatus = payload.data?.status === 'ready' ? 'ready' : 'not_ready';
    const agents = payload.data?.agents ?? [];
    const agentStatus = resolveAgentStatus(agents);

    if (centerStatus !== 'ready') {
      return {
        centerStatus,
        agentStatus,
        lightTone: 'critical',
      };
    }

    if (agentStatus !== 'online') {
      return {
        centerStatus,
        agentStatus,
        lightTone: 'warning',
      };
    }

    return {
      centerStatus,
      agentStatus,
      lightTone: 'success',
    };
  } catch {
    return {
      centerStatus: 'unreachable',
      agentStatus: 'unknown',
      lightTone: 'critical',
    };
  }
}

function resolveAgentStatus(agents: Array<{ status?: string }>): AgentRuntimeStatus {
  if (agents.some((agent) => agent.status === 'online')) {
    return 'online';
  }

  if (agents.some((agent) => agent.status === 'heartbeat_timeout')) {
    return 'heartbeat_timeout';
  }

  return 'unknown';
}
