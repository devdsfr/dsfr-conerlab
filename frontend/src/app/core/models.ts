export interface League {
  id: number;
  name: string;
  country: string;
  tier: string;
}

export interface Season {
  id: number;
  league_id: number;
  year: number;
  label: string;
}

export interface Team {
  id: number;
  name: string;
  short_name: string;
  country: string;
  tier: string;
}

export interface StatSummary {
  count: number;
  mean: number;
  max: number;
  min: number;
  std_dev: number;
  variance: number;
  median: number;
  mode: number[];
  total: number;
  coefficient_of_variation: number;
  consistency_index: number;
}

export interface FrequencyResult {
  threshold: number;
  count: number;
  total: number;
  pct: number;
}

// Estatísticas complementares por partida (posse, chutes, cartões etc.) — já
// reorientadas pela perspectiva da equipe consultada (For = a própria equipe,
// Against = o adversário), mesmo padrão de corners_for/corners_against. Undefined/
// null quando o provedor não publicou aquele campo para a partida (comum em ligas
// menores) — ver domain.TeamMatchView no backend.
export interface TeamMatchView {
  match_id: number;
  match_date: string;
  opponent: Team;
  is_home: boolean;
  corners_for: number;
  corners_against: number;
  total_corners: number;
  opponent_tier: string;

  possession_for?: number | null;
  possession_against?: number | null;
  shots_for?: number | null;
  shots_against?: number | null;
  shots_on_target_for?: number | null;
  shots_on_target_against?: number | null;
  shots_insidebox_for?: number | null;
  shots_insidebox_against?: number | null;
  shots_outsidebox_for?: number | null;
  shots_outsidebox_against?: number | null;
  blocked_shots_for?: number | null;
  blocked_shots_against?: number | null;
  fouls_for?: number | null;
  fouls_against?: number | null;
  offsides_for?: number | null;
  offsides_against?: number | null;
  yellow_cards_for?: number | null;
  yellow_cards_against?: number | null;
  red_cards_for?: number | null;
  red_cards_against?: number | null;
}

export interface SplitStats {
  sample_size: number;
  mean: number;
  max: number;
  min: number;
  consistency: number;
}

export interface DashboardResult {
  team: Team;
  sample_size: number;
  period: string;
  recent_matches: TeamMatchView[];
  corners_for: StatSummary;
  corners_against: StatSummary;
  total_corners: StatSummary;
  balance: number;
  frequencies: FrequencyResult[];
  trend: number[];
  home_stats?: SplitStats;
  away_stats?: SplitStats;
}

export interface TeamComparisonSide {
  team: Team;
  sample_size: number;
  total_corners: StatSummary;
  corners_for: StatSummary;
  corners_against: StatSummary;
  home?: SplitStats;
  away?: SplitStats;
  trend: number[];
}

export interface ComparisonResult {
  period: string;
  team_a: TeamComparisonSide;
  team_b: TeamComparisonSide;
}

export interface FilterRunRequest {
  league_id: number;
  season_ids: number[];
  team_id?: number | null;
  last_n_games?: number;
  home_away?: string;
  corners_threshold: number;
  opponent_tier?: string;
  max_odds?: number;
  stake?: number;
}

export interface BacktestEntry {
  match_id: number;
  match_date: string;
  team: string;
  opponent: string;
  is_home: boolean;
  total_corners: number;
  hit: boolean;
  odd: number;
  profit_loss: number;
}

export interface BacktestResult {
  criteria: FilterRunRequest;
  period: string;
  match_count: number;
  hits: number;
  misses: number;
  hit_rate: number;
  miss_rate: number;
  average_corners: number;
  longest_win_streak: number;
  longest_lose_streak: number;
  max_drawdown: number;
  total_staked: number;
  profit: number;
  roi: number;
  yield: number;
  entries: BacktestEntry[];
  disclaimer: string;
  // Plano gratuito limita o backtest aos últimos N dias (ver
  // ESTRATEGIA-MONETIZACAO.md e FilterHandler.FreeHistoryCapDays no backend).
  history_capped: boolean;
  history_cap_days?: number;
}

// Painel "Integrações" — status/consumo das APIs externas (OpenAI, API-Football, SportMonks)
export interface DailyCount {
  date: string;
  count: number;
}

export interface ProviderSummary {
  provider: string;
  display_name: string;
  configured: boolean;
  total_calls: number;
  success_calls: number;
  error_calls: number;
  tokens_total: number;
  last_call_at: string | null;
  last_success_at: string | null;
  last_error_at: string | null;
  last_error_message: string;
  daily_calls: DailyCount[];
}

export interface UsageSummaryResponse {
  providers: ProviderSummary[];
}

export interface TestConnectionResult {
  provider: string;
  ok: boolean;
  message: string;
  latency_ms: number;
}

export interface UsageEntry {
  provider: string;
  endpoint: string;
  success: boolean;
  status_code: number | null;
  tokens_total: number | null;
  error_message: string;
  duration_ms: number;
  created_at: string;
}

// Autenticação (necessária para o Módulo de Gestão Evolutiva de Banca, que é por
// usuário — reaproveita os mesmos endpoints usados pelo Módulo de Apostas/Alertas)
export interface AuthUser {
  id: number;
  name: string;
  email: string;
}

export interface AuthResponse {
  user: AuthUser;
  token: string;
}

// Módulo de Gestão Evolutiva de Banca
export interface BankrollPhase {
  id: number;
  user_id: number;
  sequence: number;
  name: string;
  amount: number;
}

export interface BankrollCriteria {
  user_id: number;
  min_days: number;
  min_bets: number;
  min_win_rate: number;
  min_roi: number;
  min_yield: number;
  require_positive_profit: boolean;
  min_completed_cycles: number;
  cycle_win_streak: number;
}

export interface BankrollMetrics {
  sample_size: number;
  win_rate: number;
  roi: number;
  yield: number;
  net_profit: number;
  completed_cycles: number;
  days_in_phase: number;
  max_drawdown: number;
  max_drawdown_pct: number;
  monthly_consistency: number;
}

export interface BankrollChecklistItem {
  label: string;
  met: boolean;
  current: string;
  required: string;
}

export interface BankrollMaturity {
  score: number;
  stars: number;
  status: string;
}

export interface BankrollState {
  user_id: number;
  current_phase_sequence: number;
  phase_started_at: string;
  highest_phase_sequence: number;
  promotions: number;
  demotions: number;
}

export interface BankrollStatus {
  current_phase: BankrollPhase;
  next_phase: BankrollPhase | null;
  previous_phase: BankrollPhase | null;
  metrics: BankrollMetrics;
  criteria: BankrollCriteria;
  checklist: BankrollChecklistItem[];
  ready_to_promote: boolean;
  blocked_reasons: string[];
  maturity: BankrollMaturity;
  progress: number;
  state: BankrollState;
  demotion_suggested: boolean;
  demotion_reason: string;
}

export interface BankrollHistoryEntry {
  id: number;
  user_id: number;
  from_amount: number;
  to_amount: number;
  direction: 'promotion' | 'demotion';
  reason: string;
  notes: string;
  created_at: string;
}

// Resultado do botão "Sincronizar agora" (painel Integrações) — dispara o mesmo
// ciclo de descoberta + atualização que o Render Cron Job roda periodicamente.
export interface SyncRunResult {
  discovery: {
    Targets: number;
    FixturesFound: number;
    FixturesUpserted: number;
    Errors: number;
  };
  update: {
    Checked: number;
    Finalized: number;
    StillOpen: number;
    Errors: number;
  };
  duration_ms: number;
}

// Última execução de sincronização registrada em sync_runs (manual ou via Render
// Cron Job) — usado para mostrar "Última sincronização: DD/MM HH:mm" no painel
// Integrações, independente de estado local do navegador.
export interface SyncRun {
  id: number;
  triggered_by: string;
  targets: number;
  fixtures_found: number;
  fixtures_upserted: number;
  matches_checked: number;
  matches_finalized: number;
  errors: number;
  duration_ms: number;
  created_at: string;
}

export interface SyncStatusResponse {
  last_run: SyncRun | null;
}

// Registro de rodadas confirmadas manualmente (saldo real acumulado) — ver
// BankrollComponent, aba "Rodadas".
export interface BankrollRound {
  id: number;
  user_id: number;
  phase_sequence: number;
  phase_name: string;
  result: number;
  balance_after: number;
  notes: string;
  confirmed_at: string;
}

// Assinatura Premium (Stripe) — ver ESTRATEGIA-MONETIZACAO.md
export interface BillingStatus {
  plan: string;
  subscription_status: string;
  is_premium: boolean;
  trial_ends_at?: string;
  current_period_end?: string;
  // configured=false quando o backend ainda não tem STRIPE_SECRET_KEY/STRIPE_PRICE_ID
  // configuradas — o frontend usa isso para mostrar "em breve" em vez do botão de
  // assinar (ver pkg/config/config.go e ESTRATEGIA-MONETIZACAO.md).
  configured: boolean;
}
