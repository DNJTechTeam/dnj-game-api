// Mapa de rotas exercitadas. `template` casa com o campo `route` (c.FullPath) do
// log http_request_completed no CloudWatch; `path` é o path concreto enviado.
// Mantido separado para as tabelas do relatório baterem 1:1 com o Logs Insights.
export const ROUTES = {
  // Públicas
  healthcheck: { template: '/v2/healthcheck', path: '/healthcheck' },
  rankingsIndividual: { template: '/v2/rankings', path: '/rankings?scope=individual&page=1' },
  rankingsGroups: { template: '/v2/rankings', path: '/rankings?scope=groups&page=1' },
  liveDisplayTv: { template: '/v2/live-display', path: '/live-display?target=tv' },
  scheduleHome: { template: '/v2/schedule', path: '/schedule?view=home' },

  // Participante autenticado (polling do shell)
  specialEventsActive: { template: '/v2/special-events/active', path: '/special-events/active?target=app' },
  notifications: { template: '/v2/notifications', path: '/notifications' },
  challenges: { template: '/v2/activities', path: '/activities?kind=challenge' },
  gameOverview: { template: '/v2/game/overview', path: '/game/overview' },

  // Tela game
  currentRun: { template: '/v2/activity-runs/current', path: '/activity-runs/current' },
  currentParticipation: { template: '/v2/participations/current', path: '/participations/current' },

  // Feed / conta
  momentsFeed: { template: '/v2/moments', path: '/moments?scope=feed' },
  session: { template: '/v2/auth/session', path: '/auth/session' },

  // Gestor
  managerOverview: { template: '/v2/manager/game-overview', path: '/manager/game-overview' },
};
