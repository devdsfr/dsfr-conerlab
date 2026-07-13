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

export interface TeamMatchView {
  match_id: number;
  match_date: string;
  opponent: Team;
  is_home: boolean;
  corners_for: number;
  corners_against: number;
  total_corners: number;
  opponent_tier: string;
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
}
