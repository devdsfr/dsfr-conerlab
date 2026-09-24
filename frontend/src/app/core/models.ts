export interface League {
  id: number;
  name: string;
  country: string;
  tier: string;
}

// Próximo jogo mapeado (status AGENDADO) — alimenta o calendário da página "Visão
// Geral", a tela inicial do app.
export interface UpcomingMatch {
  match_id: number;
  match_date: string;
  league_id: number;
  /** Temporada à qual esta partida pertence. Vai junto na navegação para o
   * Dashboard: sem ela o Dashboard adivinhava a temporada e podia acabar
   * mostrando outra equipe (ver openTeamDashboard). */
  season_id: number;
  league_name: string;
  round: number;
  home_team_id: number;
  home_team_name: string;
  away_team_id: number;
  away_team_name: string;
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

  goals_for: number;
  goals_against: number;
  total_goals: number;

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
  /** Quantidade de partidas que REALMENTE sustentam os números abaixo. */
  sample_size: number;
  /** Descrição da amostra real (ver describePeriod no backend). */
  period: string;
  /** Janela pedida pelo usuário (5/10/15/20) — pode ser maior que sample_size. */
  requested_limit: number;
  recent_matches: TeamMatchView[];
  corners_for: StatSummary;
  corners_against: StatSummary;
  total_corners: StatSummary;
  balance: number;
  frequencies: FrequencyResult[];
  trend: number[];
  home_stats?: SplitStats;
  away_stats?: SplitStats;

  // Análise de gols — espelho dos escanteios (ver aba "Gols" no Dashboard).
  goals_for: StatSummary;
  goals_against: StatSummary;
  total_goals: StatSummary;
  goal_balance: number;
  goal_frequencies: FrequencyResult[];
  goal_trend: number[];
  goal_home_stats?: SplitStats;
  goal_away_stats?: SplitStats;

  // Análise de impedimentos — calculada só sobre jogos com o dado (offside_sample_size
  // pode ser menor que sample_size; 0 = sem dados de impedimento).
  offsides_for: StatSummary;
  offsides_against: StatSummary;
  total_offsides: StatSummary;
  offside_balance: number;
  offside_frequencies: FrequencyResult[];
  offside_trend: number[];
  offside_home_stats?: SplitStats;
  offside_away_stats?: SplitStats;
  offside_sample_size: number;

  // Chutes (total) — nullable, amostra própria (shot_sample_size).
  shots_for: StatSummary;
  shots_against: StatSummary;
  total_shots: StatSummary;
  shot_balance: number;
  shot_frequencies: FrequencyResult[];
  shot_trend: number[];
  shot_home_stats?: SplitStats;
  shot_away_stats?: SplitStats;
  shot_sample_size: number;

  // Chutes no gol — idem (sot_sample_size).
  shots_on_target_for: StatSummary;
  shots_on_target_against: StatSummary;
  total_shots_on_target: StatSummary;
  sot_balance: number;
  sot_frequencies: FrequencyResult[];
  sot_trend: number[];
  sot_home_stats?: SplitStats;
  sot_away_stats?: SplitStats;
  sot_sample_size: number;
}

/** Ponto da evolução COM identidade: sem match_id, data e adversário não há
 * como auditar o que o gráfico desenha. */
export interface MatchPoint {
  match_id: number;
  date: string;
  opponent_id: number;
  opponent_name: string;
  is_home: boolean;
  /** null = o provedor não publicou a métrica nesta partida (não é zero). */
  value: number | null;
  goals_for: number;
  goals_against: number;
}

export interface FrequencyBand {
  threshold: number;
  hits: number;
  sample: number;
  percentage: number;
}

/** Um valor observado e quantas partidas tiveram exatamente esse valor.
 * `sample` é metric_sample_size (partidas COM a métrica), não sample_size. */
export interface DistributionBucket {
  value: number;
  count: number;
  sample: number;
  percentage: number;
}

export interface TeamComparisonSide {
  team: Team;
  /** Partidas do recorte (liga+temporada+local) desta equipe. */
  sample_size: number;
  /** Quantas dessas partidas têm a métrica escolhida — pode ser menor. */
  metric_sample_size: number;
  metric_available: boolean;
  /** Descrição da amostra real desta equipe — os dois lados podem divergir. */
  period: string;
  summary: StatSummary;
  frequencies: FrequencyBand[] | null;
  /** Distribuição observada, ordenada por valor crescente. */
  distribution: DistributionBucket[];
  evolution: MatchPoint[];
}

export interface H2HMatch {
  match_id: number;
  date: string;
  league_id: number;
  season_id: number;
  home_team_id: number;
  home_team_name: string;
  away_team_id: number;
  away_team_name: string;
  home_goals: number;
  away_goals: number;
  home_value: number | null;
  away_value: number | null;
}

export interface HeadToHead {
  match_count: number;
  matches: H2HMatch[];
}

export type ComparatorVenue = 'geral' | 'casa' | 'fora';
export type ComparatorMetric = 'corners' | 'goals' | 'shots' | 'shots_on_target' | 'offsides';
export type ComparatorPerspective = 'produzido' | 'concedido' | 'total';

export interface ComparisonResult {
  /** Janela PEDIDA, rotulada como pedido. A amostra real de cada equipe está
   * em team_a.period / team_b.period — elas podem ser diferentes. */
  period: string;
  requested_limit: number;
  venue: ComparatorVenue;
  metric: ComparatorMetric;
  perspective: ComparatorPerspective;
  /** false quando não há definição de faixas para métrica+perspectiva. */
  frequencies_available: boolean;
  frequencies_note?: string;
  team_a: TeamComparisonSide;
  team_b: TeamComparisonSide;
  h2h: HeadToHead;
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
  // Métrica alternativa: cada uma com seu threshold; não-escanteios usam fixed_odd.
  metric?: string;
  goals_threshold?: number;
  offsides_threshold?: number;
  shots_threshold?: number;
  shots_on_target_threshold?: number;
  fixed_odd?: number;
}

export interface BacktestEntry {
  match_id: number;
  match_date: string;
  team: string;
  opponent: string;
  is_home: boolean;
  total_corners: number;
  total_goals: number;
  total_offsides: number;
  total_shots: number;
  total_shots_on_target: number;
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
  average_goals: number;
  average_offsides: number;
  average_shots: number;
  average_shots_on_target: number;
  metric: string;
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

  // Procedência das odds usadas (AUD-001). 'real' = odd de mercado;
  // 'synthetic' = odd derivada do próprio histórico; 'fixed' = odd única
  // informada pelo usuário; 'none' = nenhuma odd envolvida.
  // financials_reliable só é true em 'real' — nos demais casos ROI, yield e
  // lucro descrevem um cenário hipotético e a tela precisa dizer isso.
  odds_source: 'real' | 'synthetic' | 'fixed' | 'none';
  financials_reliable: boolean;
}

// ---------------------------------------------------------------------------
// Strategy Engine (Remodelagem F4/F5) — estratégias persistidas com backtest,
// health e scores proprietários calculados pelo backend (Formula Catalog).
// ---------------------------------------------------------------------------

export interface Strategy {
  id: number;
  owner_id?: number | null;
  name: string;
  description: string;
  definition: string; // JSON no formato do Simulador (FilterRunRequest)
  origin: string; // 'user' | 'discovery'
  visibility: string;
  active: boolean;
  favorite: boolean;
  created_at: string;
  updated_at: string;
}

export interface StrategyBacktest {
  id: number;
  strategy_id: number;
  games: number;
  wins: number;
  losses: number;
  voids: number;
  roi?: number | null;
  yield?: number | null;
  ev?: number | null;
  drawdown?: number | null;
  profit?: number | null;
  confidence?: number | null;
  created_at: string;
}

export interface StrategyHealth {
  strategy_id: number;
  health_score: number; // 0..100 (50 = estável)
  trend?: number | null; // -1..1
  variation: string; // JSON dos deltas
  updated_at: string;
}

export interface StrategyScores {
  strategy_id: number;
  dsfr_score: number;
  components: string; // JSON dos componentes normalizados
  confidence?: number | null;
  robustness?: number | null;
  volatility?: number | null;
  risk?: number | null;
  ranking?: number | null;
  lifecycle_stage: string; // nascimento|crescimento|maturidade|declinio|obsoleta
  updated_at: string;
}

export interface StrategyBundle {
  strategy: Strategy;
  health?: StrategyHealth | null;
  scores?: StrategyScores | null;
  backtests: StrategyBacktest[];
}

// ---------------------------------------------------------------------------
// Strategy Discovery Engine (Remodelagem F6 — doc 08): estratégias que o próprio
// sistema encontrou minerando o histórico. O backend já entrega a linha do
// ranking achatada (estratégia + backtest + health + scores em um objeto só),
// para o frontend não reimplementar nenhuma regra de negócio.
// ---------------------------------------------------------------------------

export interface DiscoveredStrategy {
  id: number;
  name: string;
  description: string;
  definition: FilterRunRequest; // objeto pronto para recarregar no Simulador

  games: number;
  wins: number;
  losses: number;
  win_rate: number;
  roi: number;
  yield: number;
  /**
   * NULO quando não calculado — que é o caso hoje (AUD-002): o EV exigiria
   * P(vitória) estimada fora da amostra, que o sistema ainda não produz.
   * Exibir como "—", nunca como 0.
   */
  ev: number | null;
  profit: number;
  drawdown: number;

  dsfr_score: number;
  /** Faixa de qualidade do doc 08: Elite | Excelente | Muito Boa | Boa | Regular. */
  classification: string;
  confidence: number;
  robustness: number;
  risk: number;
  health_score: number;
  lifecycle_stage: string;

  updated_at: string;
}

export interface DiscoveredStrategiesResponse {
  strategies: DiscoveredStrategy[];
  count: number;
  disclaimer: string;
}

/** Resumo de um ciclo de descoberta disparado manualmente. */
export interface DiscoveryRunResult {
  league_id?: number;
  league_name?: string;
  leagues?: number;
  combinations: number;
  approved?: number;
  published: number;
  deactivated: number;
  errors: number;
  /** Contagem por motivo de descarte (amostra_insuficiente, roi_baixo, ...). */
  rejections?: Record<string, number>;
}

export interface StrategyEvaluation {
  backtest: StrategyBacktest;
  health: StrategyHealth;
  scores: StrategyScores;
  raw?: BacktestResult;
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

/** Andamento do ciclo de sincronização (GET /sync/progress). O ciclo leva minutos
 *  por causa do throttle da API-Football, então a tela acompanha por polling em vez
 *  de segurar a requisição aberta. */
export interface TaskProgress<TResult = unknown> {
  running: boolean;
  phase: string;
  phase_label: string;
  current: number;
  total: number;
  percent: number;
  message: string;
  started_at: string | null;
  finished_at: string | null;
  duration_ms: number;
  error: string;
  result?: TResult;
}

/** Resultado do ciclo de sincronização, entregue no fim do acompanhamento. */
export interface SyncOutcome {
  discovery: { Targets: number; FixturesFound: number; FixturesUpserted: number; Errors: number };
  update: { Checked: number; Finalized: number; StillOpen: number; Errors: number };
}

export type SyncProgress = TaskProgress<SyncOutcome>;

/** Resultado da varredura de descobertas. */
export interface DiscoveryOutcome {
  leagues: number;
  combinations: number;
  published: number;
  deactivated: number;
  errors: number;
}

export type DiscoveryProgress = TaskProgress<DiscoveryOutcome>;

export interface SyncStartResponse {
  started: boolean;
  message: string;
  progress: SyncProgress;
}

export interface TableSize {
  name: string;
  bytes: number;
  pretty: string;
  rows: number;
}

/** Consumo de armazenamento do banco — acompanha o crescimento sem precisar abrir
 *  o console do Neon. Opcional: se a consulta falhar, o backend omite o campo e a
 *  tela simplesmente não mostra o card. */
export interface StorageUsage {
  total_bytes: number;
  total_pretty: string;
  quota_bytes: number;
  quota_pretty: string;
  used_percent: number;
  retention_days: number;
  tables: TableSize[];
}

export interface UsageSummaryResponse {
  providers: ProviderSummary[];
  storage?: StorageUsage;
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

  /**
   * 'success' = o ciclo completou as duas fases (pode ter trazido zero dado).
   * 'failed'  = o ciclo foi interrompido; error_message diz por quê.
   *
   * Um ciclo interrompido antes não deixava registro nenhum, então a tela seguia
   * anunciando a sincronização anterior como a mais recente — o sistema parecia
   * mais saudável justamente por ter falhado.
   */
  status: string;
  error_message?: string;
}

export interface SyncStatusResponse {
  /** Última TENTATIVA de sincronização — pode ter trazido zero dado. */
  last_run: SyncRun | null;

  /**
   * Último ciclo que realmente trouxe dado (sem erros e com partida gravada ou
   * finalizada). É esta data que importa para saber se o banco está em dia.
   *
   * A distinção não é preciosismo: entre 02/08 e 12/09 o cron rodou todos os
   * dias e a tela dizia "última sincronização: hoje" com o banco seis semanas
   * parado, porque a API recusava as chamadas devolvendo HTTP 200.
   */
  last_successful_run: SyncRun | null;

  /** Horas desde o último ciclo bem-sucedido. null = nunca houve um. */
  hours_since_success: number | null;

  /** true quando passou do limite (48h) ou nunca houve sincronização bem-sucedida. */
  stale: boolean;

  /**
   * Último ciclo AUTOMÁTICO (Render Cron Job) e último ciclo MANUAL (botão
   * "Sincronizar agora"), separados.
   *
   * Sem essa separação a tela não consegue responder "o worker rodou?": ela
   * mostrava o ciclo mais recente de qualquer origem, então um clique manual
   * deixava tudo com cara de saudável. Foi assim que o fato de o Cron Job NUNCA
   * ter sido criado no Render passou despercebido.
   */
  last_cron_run: SyncRun | null;
  last_manual_run: SyncRun | null;

  /** Horas desde o último ciclo automático. null = nunca rodou. */
  hours_since_cron: number | null;

  /** true quando o ciclo automático passou de 36h ou nunca rodou. */
  cron_stale: boolean;

  /**
   * true quando NENHUM ciclo automático foi registrado. Diferente de
   * cron_stale sozinho: "nunca rodou" quase sempre significa Cron Job
   * inexistente ou mal configurado, e esperar não resolve.
   */
  cron_never_ran?: boolean;

  /** Mensagem do erro mais recente do provedor, quando houver um nos últimos 7 dias. */
  provider_error?: string;
  provider_error_at?: string;
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
