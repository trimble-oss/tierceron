import 'dart:html';
import 'dart:core';
import 'dart:async';
import 'dart:convert';

import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';

import '../twirp_requests.dart';
import '../init_service.dart';
import '../log_dialog/log_dialog_component.dart';
// Displayed when vault has not yet been created,w

@Component(
  selector: 'vault-start',
  styleUrls: ['vault_start_component.css'],
  templateUrl: 'vault_start_component.html',
  directives: const [coreDirectives, 
                     formDirectives,
                     LogDialogComponent],
  pipes: const [SlicePipe],
  providers: const [InitService]
)

class VaultStartComponent implements OnInit{
  final InitService _initService;

  String LogData;
  bool DialogVisible;
  final List<String> Envs = ['dev', 'QA', 'RQA', 'staging', 'local'];  // Valid environment options
  Set<UISeedFile> Seeds;               // Seed files passed to vault
  //int 
  @Input()
  FileList SeedBuffer;  // MODEL: <input> for file upload under #file_list
  @Input()
  String Username;      // MODEL: <input> for new username under login_creation
  @Input()
  String Password;      // MODEL: <input> for new password under login_creation
  @Input()
  String Environment = 'dev';

  VaultStartComponent(this._initService);
  
  Future<Null> ngOnInit() async {
    Seeds = Set.identity();
  }

  // Callback for file input element
  GetFiles(event) {
    // Ensure warning is hidden if new files have been chosen
    querySelector('#no_seed_warn').hidden = true;

    RegExp ext = new RegExp(r'(.+\.yml|.+\.yaml)'); 
    this.SeedBuffer = event.target.files; // Get files from html element

    SeedBuffer.forEach((bufferfile){
      // Make sure file is yaml
      if (ext.hasMatch(bufferfile.name)) {;
        bool fileExists = false;
        Seeds.forEach((seedfile) => // Skip duplicates
          fileExists = fileExists || seedfile.name == bufferfile.name
        );
        if (!fileExists){ // Add new file to list
          // Create a new file reader for this file
          FileReader f = new FileReader();
          f.onLoadEnd.listen((fileEvent) {
          // Convert to base64, fetch file name and environment
            List<int> fileBytes = utf8.encode(f.result);
            Seeds.add(new UISeedFile(bufferfile.name, base64Encode(fileBytes)));
          });
          // Log error events to the console
          f.onError.listen((errorEvent) => print(errorEvent));
          f.readAsText(bufferfile);
        }
      }
    });

  }

  // Used for remove file button
  RemoveFile(UISeedFile seed) {
    Seeds.remove(seed);
  }

  // Used to send seed files and start vault
  StartVault() async{
    bool valid = true;
    if(Username == null || Username.length == 0){ // Check username exists
      valid = false;
      querySelector('#username').classes.add('input-error');
    } 
    if(Password == null || Password.length == 0) { // Check password exists
      valid = false;
      querySelector('#password').classes.add('input-error');
    }
    if(Seeds == null || Seeds.length == 0) { // Check at least one seed file given
      valid = false;
      querySelector('#no_seed_warn').hidden = false;
    }
    if(valid){ // Username, Password, files given; Begin init

      // Proceed to seed vault and change layout
      // Iterate through files, converting to base 64

      List<Map> files = new List<Map>();
      Seeds.forEach((file) {
        files.add({
          'env' : (querySelector('#' + file.name.substring(0, file.name.length-4)) 
                  as SelectElement).value,
          'data' : file.data
        });
      });

      

      Map<String, dynamic> initRequest = new Map();
      initRequest['files'] = files;
      initRequest['username'] = Username;
      initRequest['password'] = Password;
      initRequest['env'] = Environment;

      // MaKe request
      _initService.MakeRequest(initRequest).then((resp){
        if (resp['err'] == true){
          // Reroute
        }
        LogData = '<p>' + resp['log'].replaceAll('\n', '<br />') +'</p>';
        for (var token in resp['tokens']) {
          if (token['name'] == 'Auth'){
            window.localStorage['Token'] = token['value'];
            continue;
          }
          print(token['name'] + ': ' + token['value']);
        }

        
      });
      DialogVisible = true;

      // Redirect if successful
    }
  }

  // Remove error formatting from username/password box
  Future<Null> UnRedify(event) async{
    (event.target as Element).classes.remove('input-error');
  }

}
