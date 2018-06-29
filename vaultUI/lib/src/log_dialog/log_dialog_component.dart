import 'package:angular/angular.dart';
import 'package:angular_components/angular_components.dart';

@Component(
  selector: 'log-dialog',
  templateUrl: 'log_dialog_component.html',
  styleUrls: ['log_dialog_component.css'],
  directives: const [coreDirectives,  
                     MaterialDialogComponent, 
                     ModalComponent],
  providers: const [materialProviders]
)

class LogDialogComponent{
  @Input()
  bool DialogVisible;
  @Input()
  String LogData;
}