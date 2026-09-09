package repositories

// managerScopeActivityPredicate centralizes authorization for operational
// managers. Managers are assigned to an area, never to an individual activity.
const managerScopeActivityPredicate = `
    users.id = ? AND (
        (users.manager_scope IS NULL AND activities.kind IN ('checkpoint','challenge','competitive','live','schedule')) OR
        (users.manager_scope = 'actions' AND activities.kind IN ('checkpoint','challenge','competitive')) OR
        (users.manager_scope = 'space' AND activities.kind = 'schedule') OR
        (users.manager_scope = 'special_events' AND activities.kind = 'live')
    )`

const managerScopeRunPredicate = `
    users.id = ? AND (
        (users.manager_scope IS NULL AND activities.kind IN ('checkpoint','challenge','competitive','live')) OR
        (users.manager_scope = 'actions' AND activities.kind IN ('checkpoint','challenge','competitive')) OR
        (users.manager_scope = 'special_events' AND activities.kind = 'live')
    )`
