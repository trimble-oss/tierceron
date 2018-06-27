import 'package:angular/angular.dart';
import 'package:vaultUI/app_component.template.dart' as ng;
import 'dart:html';

import 'main.template.dart' as self;

@GenerateInjector([
  ClassProvider(HttpRequest, useClass: HttpRequest)
])
final InjectorFactory injector = self.injector$Injector;

void main() {
  runApp(ng.AppComponentNgFactory, createInjector: injector);
}
