import com.sun.jna.*;
import java.util.*;

public class Client {
  public interface TemplatePopulator extends Library {
        public class GoString extends Structure {
            public static class ByValue extends GoString implements Structure.ByValue {}
            public String p;
            public long n;
            protected List getFieldOrder(){
                return Arrays.asList(new String[]{"p","n"});
            }

        }
    public String ConfigTemplateLib(GoString.ByValue token, GoString.ByValue address, GoString.ByValue certPath, GoString.ByValue env, GoString.ByValue templatePath, GoString.ByValue configuredFilePath, boolean secretMode, GoString.ByValue servicesWanted);
  }
  static public void main(String argv[]) {
    //String[] services = {"ST/"};
    TemplatePopulator populator = (TemplatePopulator) Native.loadLibrary(
      "./templatePopulator.so", TemplatePopulator.class);

    TemplatePopulator.GoString.ByValue services = new TemplatePopulator.GoString.ByValue();
    services.p = "ServiceName/";
    services.n = services.p.length();

    TemplatePopulator.GoString.ByValue token = new TemplatePopulator.GoString.ByValue();
    token.p = "<randomtoken>";
    token.n = token.p.length();

    TemplatePopulator.GoString.ByValue addr = new TemplatePopulator.GoString.ByValue();
    addr.p = "https://<vaulthostandport>";
    addr.n = addr.p.length();

    TemplatePopulator.GoString.ByValue certPath = new TemplatePopulator.GoString.ByValue();
    certPath.p = "~/workspace/VaultConfig.Bootstrap/certs/cert_files/serv_cert.pem";
    certPath.n = certPath.p.length();

    TemplatePopulator.GoString.ByValue tempDir = new TemplatePopulator.GoString.ByValue();
    tempDir.p = "~/workspace/VaultConfig.Bootstrap/trc_templates/ST/hibernate.properties.tmpl";
    tempDir.n = tempDir.p.length();

    TemplatePopulator.GoString.ByValue endDir = new TemplatePopulator.GoString.ByValue();
    endDir.p = "~/workspace/VaultConfig.Bootstrap/config_files/ST/hibernate.properties";
    endDir.n = endDir.p.length();

    TemplatePopulator.GoString.ByValue env = new TemplatePopulator.GoString.ByValue();
    env.p = "dev";
    env.n = env.p.length();

    System.out.printf(populator.ConfigTemplateLib(token, addr, certPath, tempDir, endDir, env, false, services));
  }
}
