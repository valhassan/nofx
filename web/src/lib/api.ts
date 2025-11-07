import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  AIModel,
  Exchange,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
} from '../types'

const API_BASE = '/api'

// Helper function to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  return headers
}

export const api = {
  // AI trader management API
  async getTraders(): Promise<TraderInfo[]> {
    const res = await fetch(`${API_BASE}/my-traders`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get trader list')
    return res.json()
  },

  // Get public trader list (no authentication required)
  async getPublicTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/traders`)
    if (!res.ok) throw new Error('Failed to get public trader list')
    return res.json()
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const res = await fetch(`${API_BASE}/traders`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to create trader')
    return res.json()
  },

  async deleteTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to delete trader')
  },

  async startTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/start`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to start trader')
  },

  async stopTrader(traderId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/stop`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to stop trader')
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/prompt`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify({ custom_prompt: customPrompt }),
    })
    if (!res.ok) throw new Error('Failed to update custom strategy')
  },

  async getTraderConfig(traderId: string): Promise<any> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/config`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get trader configuration')
    return res.json()
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const res = await fetch(`${API_BASE}/traders/${traderId}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update trader')
    return res.json()
  },

  // AI model configuration API
  async getModelConfigs(): Promise<AIModel[]> {
    const res = await fetch(`${API_BASE}/models`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get model configuration')
    return res.json()
  },

  // Get system supported AI models list (no authentication required)
  async getSupportedModels(): Promise<AIModel[]> {
    const res = await fetch(`${API_BASE}/supported-models`)
    if (!res.ok) throw new Error('Failed to get supported models')
    return res.json()
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    const res = await fetch(`${API_BASE}/models`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update model configuration')
  },

  // Exchange configuration API
  async getExchangeConfigs(): Promise<Exchange[]> {
    const res = await fetch(`${API_BASE}/exchanges`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get exchange configuration')
    return res.json()
  },

  // Get system supported exchanges list (no authentication required)
  async getSupportedExchanges(): Promise<Exchange[]> {
    const res = await fetch(`${API_BASE}/supported-exchanges`)
    if (!res.ok) throw new Error('Failed to get supported exchanges')
    return res.json()
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/exchanges`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(request),
    })
    if (!res.ok) throw new Error('Failed to update exchange configuration')
  },

  // Get system status (supports trader_id)
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get system status')
    return res.json()
  },

  // Get account information (supports trader_id)
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const res = await fetch(url, {
      cache: 'no-store',
      headers: {
        ...getAuthHeaders(),
        'Cache-Control': 'no-cache',
      },
    })
    if (!res.ok) throw new Error('Failed to get account information')
    const data = await res.json()
    console.log('Account data fetched:', data)
    return data
  },

  // Get positions list (supports trader_id)
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get positions list')
    return res.json()
  },

  // Get decision logs (supports trader_id)
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get decision logs')
    return res.json()
  },

  // Get latest decisions (supports trader_id)
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get latest decisions')
    return res.json()
  },

  // Get statistics (supports trader_id)
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get statistics')
    return res.json()
  },

  // Get equity history data (supports trader_id)
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get historical data')
    return res.json()
  },

  // Batch get historical data for multiple traders (no authentication required)
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const res = await fetch(`${API_BASE}/equity-history-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ trader_ids: traderIds }),
    })
    if (!res.ok) throw new Error('Failed to get batch historical data')
    return res.json()
  },

  // Get top 5 traders data (no authentication required)
  async getTopTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/top-traders`)
    if (!res.ok) throw new Error('Failed to get top 5 traders')
    return res.json()
  },

  // Get public trader configuration (no authentication required)
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const res = await fetch(`${API_BASE}/trader/${traderId}/config`)
    if (!res.ok) throw new Error('Failed to get public trader configuration')
    return res.json()
  },

  // Get AI learning performance analysis (supports trader_id)
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    const res = await fetch(url, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get AI learning data')
    return res.json()
  },

  // Get competition data (no authentication required)
  async getCompetition(): Promise<CompetitionData> {
    const res = await fetch(`${API_BASE}/competition`)
    if (!res.ok) throw new Error('Failed to get competition data')
    return res.json()
  },

  // User signal source configuration API
  async getUserSignalSource(): Promise<{
    coin_pool_url: string
    oi_top_url: string
  }> {
    const res = await fetch(`${API_BASE}/user/signal-sources`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get user signal source configuration')
    return res.json()
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    const res = await fetch(`${API_BASE}/user/signal-sources`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({
        coin_pool_url: coinPoolUrl,
        oi_top_url: oiTopUrl,
      }),
    })
    if (!res.ok) throw new Error('Failed to save user signal source configuration')
  },

  // Get server IP (requires authentication, for whitelist configuration)
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const res = await fetch(`${API_BASE}/server-ip`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get server IP')
    return res.json()
  },
}
