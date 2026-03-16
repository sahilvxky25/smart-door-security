import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:provider/provider.dart';
import 'config/app_config.dart';
import 'models/event.dart';
import 'providers/auth_provider.dart';
import 'providers/call_provider.dart';
import 'providers/door_provider.dart';
import 'providers/event_provider.dart';
import 'providers/family_provider.dart';
import 'providers/signaling_provider.dart';
import 'screens/call_screen.dart';
import 'screens/event_detail_screen.dart';
import 'screens/events_screen.dart';
import 'screens/family_screen.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';
import 'screens/profile_screen.dart';
import 'screens/settings_screen.dart';
import 'services/api_service.dart';
import 'services/signaling_service.dart';
import 'services/webrtc_service.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Load config from SharedPreferences
  final config = await AppConfig.load();

  // Initialize local notifications
  const AndroidInitializationSettings androidInit =
      AndroidInitializationSettings('@mipmap/ic_launcher');
  const InitializationSettings initSettings = InitializationSettings(
    android: androidInit,
  );
  final notifications = FlutterLocalNotificationsPlugin();
  await notifications.initialize(
    initSettings,
    // Navigate to call screen when user taps an incoming-visitor notification.
    onDidReceiveNotificationResponse: (details) {
      MyApp.navigatorKey.currentState?.pushNamed('/call');
    },
  );

  runApp(MyApp(config: config, notifications: notifications));
}

class MyApp extends StatelessWidget {
  final AppConfig config;
  final FlutterLocalNotificationsPlugin notifications;

  const MyApp({super.key, required this.config, required this.notifications});

  static final GlobalKey<NavigatorState> navigatorKey =
      GlobalKey<NavigatorState>();

  @override
  Widget build(BuildContext context) {
    final apiService = ApiService(baseUrl: config.baseUrl);
    final signalingService = SignalingService();
    final webrtcService = WebRTCService(
      signaling: signalingService,
      config: config,
    );

    return MultiProvider(
      providers: [
        Provider<AppConfig>(create: (_) => config),
        Provider<ApiService>(create: (_) => apiService),
        Provider<SignalingService>(create: (_) => signalingService),
        Provider<WebRTCService>(create: (_) => webrtcService),
        ChangeNotifierProvider<AuthProvider>(create: (_) => AuthProvider()),
        ChangeNotifierProvider<EventProvider>(
          create: (_) => EventProvider(api: apiService),
        ),
        ChangeNotifierProvider<DoorProvider>(
          create: (_) => DoorProvider(api: apiService),
        ),
        ChangeNotifierProvider<SignalingProvider>(
          create: (_) => SignalingProvider(
            service: signalingService,
            notifications: notifications,
            navigatorKey: MyApp.navigatorKey,
          ),
        ),
        ChangeNotifierProvider<CallProvider>(
          create: (context) =>
              CallProvider(webrtc: webrtcService, signaling: signalingService),
        ),
        ChangeNotifierProvider<FamilyProvider>(
          create: (_) => FamilyProvider(api: apiService),
        ),
      ],
      child: Builder(
        builder: (context) {
          // Restore auth session, then connect WebSocket if authenticated
          Future.microtask(() async {
            final auth = context.read<AuthProvider>();
            final signaling = context.read<SignalingProvider>();
            await auth.loadFromStorage(apiService);
            if (auth.isAuthenticated) {
              signaling.connect(config.wsUrl);
            }
          });

          return MaterialApp(
            title: 'Smart Door',
            theme: ThemeData(
              useMaterial3: true,
              colorSchemeSeed: Colors.indigo,
            ),
            navigatorKey: navigatorKey,
            // Auth gate: shows splash → login → home based on auth state
            home: Consumer<AuthProvider>(
              builder: (_, auth, __) {
                if (auth.isLoading) {
                  return const Scaffold(
                    body: Center(child: CircularProgressIndicator()),
                  );
                }
                return auth.isAuthenticated
                    ? const HomeScreen()
                    : const LoginScreen();
              },
            ),
            onGenerateRoute: _generateRoute,
          );
        },
      ),
    );
  }

  Route<dynamic> _generateRoute(RouteSettings settings) {
    switch (settings.name) {
      case '/login':
        return MaterialPageRoute(builder: (_) => const LoginScreen());
      case '/events':
        return MaterialPageRoute(builder: (_) => const EventsScreen());
      case '/event':
        final event = settings.arguments as Event;
        return MaterialPageRoute(
          builder: (_) => EventDetailScreen(event: event),
        );
      case '/call':
        return MaterialPageRoute(builder: (_) => const CallScreen());
      case '/settings':
        return MaterialPageRoute(
          builder: (_) => SettingsScreen(config: config),
        );
      case '/family':
        return MaterialPageRoute(builder: (_) => const FamilyScreen());
      case '/profile':
        return MaterialPageRoute(builder: (_) => const ProfileScreen());
      default:
        return MaterialPageRoute(builder: (_) => const HomeScreen());
    }
  }
}
