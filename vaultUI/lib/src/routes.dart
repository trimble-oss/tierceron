import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'route_paths.dart' as paths;
import 'login_box/login_box_component.template.dart' as lbct;

@Injectable()
class Routes {
  RoutePath get login => paths.login;

  final List<RouteDefinition> all = [
    RouteDefinition(
      path: paths.login.path,
      component: lbct.LoginBoxComponentNgFactory,
    ),
    //RouteDefinition.redirect(path: '', redirectTo: paths.login.toUrl(parameters: {"IsSealed" : "true"}))
  ];
}