import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'route_paths.dart' as paths;
import 'vault_vals/vault_vals_component.template.dart' as vvct;
import 'login_box/login_box_component.template.dart' as lbct;

@Injectable()
class Routes {
  RoutePath get values => paths.values;
  RoutePath get login => paths.login;

  final List<RouteDefinition> all = [
    RouteDefinition(
      path: paths.login.path,
      component: lbct.LoginBoxComponentNgFactory,
    ),
    RouteDefinition(
      path: paths.values.path,
      component: vvct.VaultValsComponentNgFactory,
    ),
    RouteDefinition.redirect(path: '', redirectTo: paths.login.toUrl())
  ];
}