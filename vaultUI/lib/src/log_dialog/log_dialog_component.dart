import 'package:angular/angular.dart';
import 'package:angular_components/angular_components.dart';
import 'package:cryptoutils/cryptoutils.dart';
import 'dart:async';

@Component(
  selector: 'log-dialog',
  templateUrl: 'log_dialog_component.html',
  styleUrls: ['log_dialog_component.css'],
  directives: const [CORE_DIRECTIVES, 
                     materialDirectives, 
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