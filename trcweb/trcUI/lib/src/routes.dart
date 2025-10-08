import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'route_paths.dart' as paths;
import 'vault_vals/vault_vals_component.template.dart' as vvct;
import 'login_box/login_box_component.template.dart' as lbct;
import 'vault_start/vault_start_component.template.dart' as vsct;
import 'active_sessions/active_sessions_component.template.dart' as asct;
import 'server_reset/server_reset_component.template.dart' as srct;

@Injectable()
class Routes {
  // We need getters to use routes so make sure we add these when we add
  // to the list of route definitions
  RoutePath get values => paths.values;
  RoutePath get login => paths.login;
  RoutePath get sealed => paths.sealed;
  RoutePath get sessions => paths.sessions;
  RoutePath get reset => paths.reset;

  // Don't put default routes in here, it should all get handled through AppComponent
  // Also the page will get stuck in an infinite loop sometimes if we put a default route here
  final List<RouteDefinition> all = [
    RouteDefinition(
      path: paths.login.path,
      component: lbct.LoginBoxComponentNgFactory,
    ),
    RouteDefinition(
      path: paths.values.path,
      component: vvct.VaultValsComponentNgFactory,
    ),
    RouteDefinition(
      path: paths.sealed.path,
      component: vsct.VaultStartComponentNgFactory,
    ),
    RouteDefinition(
      path: paths.sessions.path,
      component: asct.ActiveSessionsComponentNgFactory,
    ),
    RouteDefinition(
      path: paths.reset.path,
      component: srct.ServerResetComponentNgFactory,
    )
  ];
}