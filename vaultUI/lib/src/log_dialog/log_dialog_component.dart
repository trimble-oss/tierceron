import 'package:angular/angular.dart';
import 'package:angular_components/angular_components.dart';
import 'package:angular_router/angular_router.dart';

import '../routes.dart';

@Component(
  selector: 'log-dialog',
  templateUrl: 'log_dialog_component.html',
  styleUrls: ['log_dialog_component.css'],
  directives: const [coreDirectives,  
                     MaterialDialogComponent, 
                     ModalComponent],
  providers: const [materialProviders, ClassProvider(Routes)]
)

class LogDialogComponent{
  @Input()
  bool DialogVisible;
  @Input()
  String LogData;
  final Router _router;
  final Routes _routes;

  LogDialogComponent(this._router, this._routes);

  goToValues() {
    _router.navigate(_routes.values.toUrl());
  }
}
