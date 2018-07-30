import 'package:angular/angular.dart';
import 'package:angular_components/angular_components.dart';
import 'package:angular_router/angular_router.dart';

import '../routes.dart';

/* Log Dialog Component
 * Creates a dialog box using material design for displaying logs
 * after initializing the vault. Accepting the dialog reroutes the
 * user to the value viewing page
*/
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
  bool DialogVisible; // Dialog will appear when set to true
  @Input()
  String LogData; // LogData will be displayed in the dialog box
  final Router _router;
  final Routes _routes;

  LogDialogComponent(this._router, this._routes);

  goToValues() {
    _router.navigate(_routes.values.toUrl(), NavigationParams(reload: true));
  }
}
