import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'config/app_config.dart';
import 'config/app_theme.dart';
import 'services/fcm_service.dart';
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
import 'package:awesome_notifications/awesome_notifications.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  await Firebase.initializeApp();
  await FCMService.handleBackgroundMessage(message);
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp();
  FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);

  // Load config from SharedPreferences
  final config = await AppConfig.load();

  // Initialize Awesome Notifications
  AwesomeNotifications().initialize(
    null, // default icon (uses launcher icon)
    [
      NotificationChannel(
        channelKey: 'call_channel',
        channelName: 'Calls',
        channelDescription: 'Incoming video call notifications',
        defaultColor: const Color(0xFF9D50BB),
        ledColor: Colors.white,
        importance: NotificationImportance.Max,
        channelShowBadge: true,
        locked: true,
        defaultRingtoneType: DefaultRingtoneType.Ringtone,
      ),
      NotificationChannel(
        channelKey: 'alerts_channel',
        channelName: 'Alerts',
        channelDescription: 'Security alert notifications',
        defaultColor: const Color(0xFF9D50BB),
        ledColor: Colors.white,
        importance: NotificationImportance.High,
      ),
    ],
  );

  // Request notification permissions
  bool isAllowed = await AwesomeNotifications().isNotificationAllowed();
  if (!isAllowed) {
    await AwesomeNotifications().requestPermissionToSendNotifications();
  }

  runApp(MyApp(config: config));
}

class MyApp extends StatelessWidget {
  final AppConfig config;

  const MyApp({super.key, required this.config});

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
    final callProvider = CallProvider(
      webrtc: webrtcService,
      signaling: signalingService,
    );
    final eventProvider = EventProvider(api: apiService);

    return MultiProvider(
      providers: [
        Provider<AppConfig>(create: (_) => config),
        Provider<ApiService>(create: (_) => apiService),
        Provider<SignalingService>(create: (_) => signalingService),
        Provider<WebRTCService>(create: (_) => webrtcService),
        ChangeNotifierProvider<AuthProvider>(create: (_) => AuthProvider()),
        ChangeNotifierProvider<EventProvider>.value(value: eventProvider),
        ChangeNotifierProvider<DoorProvider>(
          create: (_) => DoorProvider(api: apiService),
        ),
        ChangeNotifierProvider<CallProvider>.value(value: callProvider),
        ChangeNotifierProvider<SignalingProvider>(
          create: (context) => SignalingProvider(
            service: signalingService,
            navigatorKey: MyApp.navigatorKey,
            callProvider: callProvider,
            eventProvider: eventProvider,
            doorProvider: context.read<DoorProvider>(),
          ),
        ),
        ChangeNotifierProvider<FamilyProvider>(
          create: (_) => FamilyProvider(api: apiService),
        ),
      ],
      child: Builder(
        builder: (context) {
          apiService.onUnauthorized = () async {
            final auth = context.read<AuthProvider>();
            final signaling = context.read<SignalingProvider>();
            signaling.disconnect();
            await auth.signOutToLogin(apiService, MyApp.navigatorKey);
          };

          // Restore auth session, then connect WebSocket if authenticated
          WidgetsBinding.instance.addPostFrameCallback((_) async {
            // Capture providers before the async gap.
            final auth = context.read<AuthProvider>();
            final signaling = context.read<SignalingProvider>();
            await auth.loadFromStorage(apiService);
            if (auth.isAuthenticated) {
              signaling.connect(config.wsUrl, userId: auth.user?.id);
              signaling.initializeFCM(apiService);
            }
          });

          return MaterialApp(
            title: 'Smart Door',
            theme: appTheme(),
            debugShowCheckedModeBanner: false,
            navigatorKey: navigatorKey,
            // Auth gate: shows splash → login → home based on auth state
            home: Consumer<AuthProvider>(
              builder: (_, auth, _) {
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
      case '/home':
        return MaterialPageRoute(builder: (_) => const HomeScreen());
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
